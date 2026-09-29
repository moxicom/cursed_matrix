package utils_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

func TestHashAndVerify(t *testing.T) {
	tests := []struct {
		name     string
		password string
		verify   string
		wantErr  error
	}{
		{name: "the same password verifies", password: "correct horse battery staple", verify: "correct horse battery staple"},
		{name: "a different password does not", password: "correct horse battery staple", verify: "Correct horse battery staple", wantErr: utils.ErrPasswordMismatch},
		{name: "an empty attempt does not", password: "correct horse battery staple", verify: "", wantErr: utils.ErrPasswordMismatch},
		{name: "unicode survives", password: "пароль с пробелами и Ω", verify: "пароль с пробелами и Ω"},
		{name: "a long password is not truncated", password: strings.Repeat("x", 200) + "tail", verify: strings.Repeat("x", 200) + "tail"},
		{name: "a long password differing at the end fails", password: strings.Repeat("x", 200) + "tail", verify: strings.Repeat("x", 200) + "tale", wantErr: utils.ErrPasswordMismatch},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := utils.HashPassword(t.Context(), tc.password)
			if err != nil {
				t.Fatalf("HashPassword: %v", err)
			}
			if strings.Contains(hash, tc.password) {
				t.Fatal("the hash contains the password")
			}

			err = utils.VerifyPassword(t.Context(), tc.verify, hash)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyPassword = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestHashIsSaltedPerCall(t *testing.T) {
	first, err := utils.HashPassword(t.Context(), "same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	second, err := utils.HashPassword(t.Context(), "same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if first == second {
		t.Fatal("two hashes of one password are identical; the salt is not random")
	}
	for _, hash := range []string{first, second} {
		if err := utils.VerifyPassword(t.Context(), "same password", hash); err != nil {
			t.Errorf("a freshly made hash does not verify: %v", err)
		}
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	valid, err := utils.HashPassword(t.Context(), "password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	tests := []struct {
		name string
		hash string
	}{
		{name: "empty", hash: ""},
		{name: "not a hash at all", hash: "plaintext"},
		{name: "wrong algorithm", hash: strings.Replace(valid, "argon2id", "argon2i", 1)},
		{name: "truncated", hash: valid[:len(valid)/2]},
		{name: "unreadable parameters", hash: strings.Replace(valid, "m=32768", "m=many", 1)},
		{name: "corrupt salt", hash: strings.Replace(valid, "$argon2id$", "$argon2id$!", 1)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := utils.VerifyPassword(t.Context(), "password", tc.hash); err == nil {
				t.Error("VerifyPassword accepted a hash it cannot read")
			}
		})
	}
}

func TestVerifyAbsentAccountAlwaysFails(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "empty", password: ""},
		{name: "plausible", password: "correct horse battery staple"},
		{name: "the dummy text itself", password: "there is no account with this name"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := utils.VerifyAbsentAccount(t.Context(), tc.password); !errors.Is(err, utils.ErrPasswordMismatch) {
				t.Errorf("VerifyAbsentAccount = %v, want ErrPasswordMismatch", err)
			}
		})
	}
}

// A hash issued with the parameters this server used before they became
// configurable must keep verifying under the new ones: the hash string carries
// its own parameters, and the ceiling stays where those hashes were issued.
func TestVerifyAcceptsHashesIssuedUnderOtherParameters(t *testing.T) {
	// Issued with 64 MiB, one pass, four lanes — the previous hard-wired values.
	legacy := "$argon2id$v=19$m=65536,t=1,p=4$MDEyMzQ1Njc4OWFiY2RlZg$" +
		legacyKey(t, "password", 64*1024, 1, 4)

	tests := []struct {
		name     string
		password string
		hash     string
		wantErr  error
	}{
		{name: "legacy hash, right password", password: "password", hash: legacy},
		{name: "legacy hash, wrong password", password: "wrong", hash: legacy, wantErr: utils.ErrPasswordMismatch},
		{
			name:     "hash asking for more memory than the ceiling",
			password: "password",
			hash:     strings.Replace(legacy, "m=65536", "m=131072", 1),
			wantErr:  errAnyButMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := utils.VerifyPassword(t.Context(), tc.password, tc.hash)
			switch {
			case tc.wantErr == nil && err != nil:
				t.Errorf("VerifyPassword: %v", err)
			case tc.wantErr == errAnyButMismatch && (err == nil || errors.Is(err, utils.ErrPasswordMismatch)):
				t.Errorf("VerifyPassword = %v, want a parameter refusal", err)
			case tc.wantErr != nil && tc.wantErr != errAnyButMismatch && !errors.Is(err, tc.wantErr):
				t.Errorf("VerifyPassword = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// errAnyButMismatch marks a case that must fail for a reason other than the
// password being wrong.
var errAnyButMismatch = errors.New("any error but a mismatch")

func legacyKey(t *testing.T, password string, memoryKiB, time uint32, threads uint8) string {
	t.Helper()
	key := argon2.IDKey([]byte(password), []byte("0123456789abcdef"), time, memoryKiB, threads, 32)
	return base64.RawStdEncoding.EncodeToString(key)
}

func TestConfigurePasswordHashingRejectsUnsafeParameters(t *testing.T) {
	// Every case leaves the process-wide hasher as it was; the one that
	// succeeds is put back afterwards.
	t.Cleanup(func() {
		if err := utils.ConfigurePasswordHashing(utils.DefaultPasswordHashParams()); err != nil {
			t.Fatalf("restoring defaults: %v", err)
		}
	})

	tests := []struct {
		name   string
		change func(p *utils.PasswordHashParams)
		wantOK bool
	}{
		{name: "defaults", change: func(*utils.PasswordHashParams) {}, wantOK: true},
		{name: "memory below the argon2 floor", change: func(p *utils.PasswordHashParams) { p.MemoryMiB = 4 }},
		{name: "no passes", change: func(p *utils.PasswordHashParams) { p.Time = 0 }},
		{name: "no lanes", change: func(p *utils.PasswordHashParams) { p.Threads = 0 }},
		{name: "ceiling below what is issued", change: func(p *utils.PasswordHashParams) { p.MaxMemoryMiB = 16 }},
		{name: "no slots", change: func(p *utils.PasswordHashParams) { p.MaxConcurrent = 0 }},
		{name: "no wait budget", change: func(p *utils.PasswordHashParams) { p.WaitBudget = 0 }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := utils.DefaultPasswordHashParams()
			tc.change(&params)
			err := utils.ConfigurePasswordHashing(params)
			if tc.wantOK && err != nil {
				t.Errorf("ConfigurePasswordHashing: %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Error("ConfigurePasswordHashing accepted parameters it should refuse")
			}
		})
	}
}
