package utils

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
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

const (
	argonKeyLen  = 32
	argonSaltLen = 16
	// argonMaxThreads is the widest lane count a stored hash may ask for,
	// whatever the current setting: hashes issued before the lanes were
	// narrowed carry p=4 and must keep verifying.
	argonMaxThreads = 4

	// Upper bounds on what configuration may ask for. They are far above any
	// sensible setting and exist for arithmetic, not policy: argon2 takes
	// memory in KiB as a uint32, so a large enough MiB figure wraps to a small
	// one — or to zero, which argon2 quietly raises to its minimum — and the
	// server would issue hashes far weaker than the file says. The pass count
	// is multiplied when stored hashes are checked and wraps the same way.
	maxPasswordHashMemoryMiB = 4096
	maxPasswordHashTime      = 64
	maxPasswordHashSlots     = 1024
)

// PasswordHashParams tunes argon2id and the queue in front of it. The memory
// is the point of the algorithm — it is what makes a GPU attack expensive — but
// it is held for the whole verification, so the cost is also ours: a hundred
// concurrent sign-ins would ask this process for a hundred times MemoryMiB.
// MaxConcurrent bounds the peak at MaxConcurrent × MemoryMiB.
//
// A hash string carries its own parameters, so lowering these later leaves
// existing passwords verifiable as long as MaxMemoryMiB still covers what they
// were issued with.
type PasswordHashParams struct {
	// MemoryMiB is the argon2 memory per hash, for hashes issued from now on.
	MemoryMiB uint32
	// Time is the number of passes over that memory.
	Time uint32
	// Threads is the number of lanes; more than the host has cores only adds
	// scheduling, not speed.
	Threads uint8
	// MaxMemoryMiB is the ceiling accepted from a stored hash. It must not be
	// lowered below what any live password was hashed with, or its owner can
	// no longer sign in.
	MaxMemoryMiB uint32
	// MaxConcurrent is how many hashes may run at once.
	MaxConcurrent int
	// WaitBudget is how long a request waits for a free slot before it is
	// refused. Waiting longer would not produce a hash any sooner; it would
	// only turn a burst into a queue of held connections, which is the shape
	// of the outage the attacker was aiming for.
	WaitBudget time.Duration
}

// DefaultPasswordHashParams is what the process uses until it is configured:
// 32 MiB with two passes sits between the OWASP floor (19 MiB, t=2) and the
// 64 MiB this server used to issue; one lane, because the target host has one
// core; two at a time, so the peak is 64 MiB.
func DefaultPasswordHashParams() PasswordHashParams {
	return PasswordHashParams{
		MemoryMiB:     32,
		Time:          2,
		Threads:       1,
		MaxMemoryMiB:  64,
		MaxConcurrent: 2,
		WaitBudget:    2 * time.Second,
	}
}

func (p *PasswordHashParams) validate() error {
	switch {
	case p.MemoryMiB < 8:
		return fmt.Errorf("memory %d MiB is below the 8 MiB argon2 floor", p.MemoryMiB)
	case p.MemoryMiB > maxPasswordHashMemoryMiB:
		return fmt.Errorf("memory %d MiB is above the %d MiB ceiling", p.MemoryMiB, maxPasswordHashMemoryMiB)
	case p.Time == 0:
		return errors.New("time must be at least 1 pass")
	case p.Time > maxPasswordHashTime:
		return fmt.Errorf("time %d is above the ceiling of %d passes", p.Time, maxPasswordHashTime)
	case p.Threads == 0:
		return errors.New("threads must be at least 1")
	case p.MaxMemoryMiB < p.MemoryMiB:
		return fmt.Errorf("max memory %d MiB is below the %d MiB being issued", p.MaxMemoryMiB, p.MemoryMiB)
	case p.MaxMemoryMiB > maxPasswordHashMemoryMiB:
		return fmt.Errorf("max memory %d MiB is above the %d MiB ceiling", p.MaxMemoryMiB, maxPasswordHashMemoryMiB)
	case p.MaxConcurrent <= 0:
		return errors.New("max concurrent must be at least 1")
	case p.MaxConcurrent > maxPasswordHashSlots:
		return fmt.Errorf("max concurrent %d is above the ceiling of %d", p.MaxConcurrent, maxPasswordHashSlots)
	case p.WaitBudget <= 0:
		return errors.New("wait budget must be positive")
	}
	return nil
}

func (p *PasswordHashParams) memoryKiB() uint32 { return p.MemoryMiB * 1024 }

func (p *PasswordHashParams) maxMemoryKiB() uint32 { return p.MaxMemoryMiB * 1024 }

// hashLimiter hands out the right to run one hash.
type hashLimiter struct {
	slots  chan struct{}
	budget time.Duration
}

func newHashLimiter(size int, budget time.Duration) *hashLimiter {
	return &hashLimiter{slots: make(chan struct{}, size), budget: budget}
}

// acquire waits for a slot, briefly. It reports ErrHashBusy rather than
// blocking on, because a queue of callers waiting to spend MemoryMiB each is
// the outage, not the protection against it.
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

// passwordHasher is one set of parameters, the queue that bounds them, and the
// dummy hash that makes an unknown username cost what a wrong password costs.
type passwordHasher struct {
	params PasswordHashParams
	slots  *hashLimiter
	// dummyHash is verified against when no such account exists, so a wrong
	// username costs the same time as a wrong password and the endpoint
	// cannot be used to discover which usernames are taken.
	dummyHash string
}

func newPasswordHasher(params PasswordHashParams) (*passwordHasher, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}
	h := &passwordHasher{
		params: params,
		slots:  newHashLimiter(params.MaxConcurrent, params.WaitBudget),
	}
	dummy, err := h.hash(context.Background(), "there is no account with this name")
	if err != nil {
		return nil, fmt.Errorf("dummy hash: %w", err)
	}
	h.dummyHash = dummy
	return h, nil
}

func (h *passwordHasher) hash(ctx context.Context, password string) (string, error) {
	if err := h.slots.acquire(ctx); err != nil {
		return "", err
	}
	defer h.slots.release()

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}

	p := &h.params
	key := argon2.IDKey([]byte(password), salt, p.Time, p.memoryKiB(), p.Threads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memoryKiB(), p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func (h *passwordHasher) verify(ctx context.Context, password, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("unrecognised hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return errors.New("unsupported argon2 version")
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return errors.New("unreadable parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return errors.New("unreadable salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return errors.New("unreadable hash")
	}

	// argon2 panics on zero passes or zero lanes, and on an empty key. A row
	// that says so is corrupt; it must fail this one sign-in, not the request
	// goroutine.
	if memory == 0 || time == 0 || threads == 0 || len(want) == 0 {
		return errors.New("unreadable parameters")
	}

	// The parameters come from our own column, so this is not a parser
	// guarding against a hostile hash — it is a guard against one row ever
	// being able to ask this process for an arbitrary amount of memory.
	p := &h.params
	if memory > p.maxMemoryKiB() || time > p.Time*8 || threads > max(argonMaxThreads, p.Threads) {
		return errors.New("hash parameters beyond what this server accepts")
	}

	if err := h.slots.acquire(ctx); err != nil {
		return err
	}
	defer h.slots.release()

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

// verifyAbsent spends the time a real verification would, and always fails.
//
// It goes through the same slots as a real verification, so a name that exists
// and a name that does not are indistinguishable under load as well as at
// rest — a guard that skipped the queue would be a timing signal of its own.
func (h *passwordHasher) verifyAbsent(ctx context.Context, password string) error {
	if err := h.verify(ctx, password, h.dummyHash); errors.Is(err, ErrHashBusy) {
		return err
	}
	return ErrPasswordMismatch
}

// The process-wide hasher. It is built on first use rather than at package
// initialisation: building one computes a dummy hash, and every binary that
// imports this package — the health probe that runs twice a minute among them
// — would otherwise spend that memory and time without ever hashing a password.
var (
	hasherMu sync.Mutex
	hasher   *passwordHasher
)

// currentHasher returns the configured hasher, or one built from the defaults
// when nothing configured it (tests, tools).
func currentHasher() *passwordHasher {
	hasherMu.Lock()
	defer hasherMu.Unlock()

	if hasher == nil {
		h, err := newPasswordHasher(DefaultPasswordHashParams())
		if err != nil {
			// A failure here would silently leave the login timing guard
			// inert, so it stops the process instead.
			panic("password hashing: " + err.Error())
		}
		hasher = h
	}
	return hasher
}

// ConfigurePasswordHashing replaces the process-wide parameters. It is meant
// to be called once at startup, before serving; hashes already in flight
// finish under the parameters they started with.
func ConfigurePasswordHashing(params PasswordHashParams) error {
	h, err := newPasswordHasher(params)
	if err != nil {
		return err
	}

	hasherMu.Lock()
	defer hasherMu.Unlock()
	hasher = h
	return nil
}

// HashPassword returns an encoded argon2id hash carrying its own parameters, so
// they can be changed later without invalidating existing passwords.
func HashPassword(ctx context.Context, password string) (string, error) {
	return currentHasher().hash(ctx, password)
}

// VerifyPassword reports whether the password produces the given hash. The
// comparison is constant-time: a timing difference would leak how much of the
// hash matched.
func VerifyPassword(ctx context.Context, password, encoded string) error {
	return currentHasher().verify(ctx, password, encoded)
}

// VerifyAbsentAccount spends the time a real verification would, and always
// fails. See passwordHasher.verifyAbsent.
func VerifyAbsentAccount(ctx context.Context, password string) error {
	return currentHasher().verifyAbsent(ctx, password)
}
