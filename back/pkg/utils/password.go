package utils

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// ErrPasswordMismatch is returned when a password does not match its hash. It
// carries no detail, because the caller must answer the same way in every
// failing case.
var ErrPasswordMismatch = errors.New("password does not match")

// ErrHashBusy is returned when every hashing slot is taken and one did not come
// free in time. It is a load signal, not a credential answer: the caller must
// report it as "too many attempts", never as a wrong password.
var ErrHashBusy = errors.New("no hashing slot came free")

// How many password hashes may run at once, and how long a request waits for a
// slot.
//
// The 64 MiB below is the point of the algorithm — it is what makes a GPU
// attack expensive — but it is held for the whole verification, so the cost is
// also ours: a hundred concurrent sign-ins would ask this process for 6.4 GiB.
// Four at a time bounds that at 256 MiB, and four is already twelve threads of
// work, so the ceiling is reached by memory long before it idles a CPU.
//
// A request that finds every slot taken waits, briefly, and is then refused.
// Waiting longer would not produce a hash any sooner; it would only turn a
// burst into a queue of held connections, which is the shape of the outage the
// attacker was aiming for.
const (
	maxConcurrentHashes = 4
	hashWaitBudget      = 2 * time.Second
)

// hashLimiter hands out the right to run one hash.
type hashLimiter struct {
	slots  chan struct{}
	budget time.Duration
}

func newHashLimiter(size int, budget time.Duration) *hashLimiter {
	return &hashLimiter{slots: make(chan struct{}, size), budget: budget}
}

// acquire waits for a slot, briefly. It reports ErrHashBusy rather than
// blocking on, because a queue of callers waiting to spend 64 MiB each is the
// outage, not the protection against it.
func (h *hashLimiter) acquire(ctx context.Context) error {
	timer := time.NewTimer(h.budget)
	defer timer.Stop()

	select {
	case h.slots <- struct{}{}:
		return nil
	case <-timer.C:
		return ErrHashBusy
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *hashLimiter) release() { <-h.slots }

var hashes = newHashLimiter(maxConcurrentHashes, hashWaitBudget)

// argon2id parameters. Memory dominates the cost of a GPU attack, so it is the
// one to raise first; 64 MiB per verification is affordable for a login
// endpoint that is rate limited and rare.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// HashPassword returns an encoded argon2id hash carrying its own parameters, so
// they can be raised later without invalidating existing passwords.
func HashPassword(ctx context.Context, password string) (string, error) {
	if err := hashes.acquire(ctx); err != nil {
		return "", err
	}
	defer hashes.release()

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether the password produces the given hash. The
// comparison is constant-time: a timing difference would leak how much of the
// hash matched.
func VerifyPassword(ctx context.Context, password, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("unrecognised hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return fmt.Errorf("unsupported argon2 version")
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return fmt.Errorf("unreadable parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("unreadable salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("unreadable hash")
	}

	// The parameters come from our own column, so this is not a parser
	// guarding against a hostile hash — it is a guard against one row ever
	// being able to ask this process for an arbitrary amount of memory.
	if memory > argonMemory || time > argonTime*8 || threads > argonThreads*2 {
		return fmt.Errorf("hash parameters beyond what this server issues")
	}

	if err := hashes.acquire(ctx); err != nil {
		return err
	}
	defer hashes.release()

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

// dummyHash is verified against when no such account exists, so a wrong
// username costs the same time as a wrong password and the endpoint cannot be
// used to discover which usernames are taken. A failure here would silently
// reopen that channel, so it stops the process instead.
var dummyHash = mustHash("there is no account with this name")

func mustHash(password string) string {
	hash, err := HashPassword(context.Background(), password)
	if err != nil {
		panic("cannot hash the dummy password, the login timing guard would be inert: " + err.Error())
	}
	return hash
}

// VerifyAbsentAccount spends the time a real verification would, and always
// fails.
//
// It goes through the same slots as a real verification, so a name that exists
// and a name that does not are indistinguishable under load as well as at
// rest — a guard that skipped the queue would be a timing signal of its own.
func VerifyAbsentAccount(ctx context.Context, password string) error {
	if err := VerifyPassword(ctx, password, dummyHash); errors.Is(err, ErrHashBusy) {
		return err
	}
	return ErrPasswordMismatch
}
