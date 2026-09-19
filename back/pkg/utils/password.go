package utils

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrPasswordMismatch is returned when a password does not match its hash. It
// carries no detail, because the caller must answer the same way in every
// failing case.
var ErrPasswordMismatch = errors.New("password does not match")

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
func HashPassword(password string) (string, error) {
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
func VerifyPassword(password, encoded string) error {
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
	hash, err := HashPassword(password)
	if err != nil {
		panic("cannot hash the dummy password, the login timing guard would be inert: " + err.Error())
	}
	return hash
}

// VerifyAbsentAccount spends the time a real verification would, and always
// fails.
func VerifyAbsentAccount(password string) error {
	_ = VerifyPassword(password, dummyHash)
	return ErrPasswordMismatch
}
