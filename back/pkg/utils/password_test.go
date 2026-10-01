package utils_test

import (
	"errors"
	"strings"
	"testing"

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

// legacyHash was issued for "password" with the parameters this server
// hard-wired before they became configurable: 64 MiB, one pass, four lanes. It
// is a frozen string, not one computed here, because the point is that rows
// already in the database keep working.
const legacyHash = "$argon2id$v=19$m=65536,t=1,p=4$MDEyMzQ1Njc4OWFiY2RlZg$JgR9hq+ROMEl3sIza5R6Qd+Pl9LbOkPZEO5ELlgNORc"

// A stored hash carries its own parameters. Ones issued under earlier settings
// must keep verifying; ones that ask for more than the ceiling, or for values
// argon2 would panic on, must be refused as unreadable rather than as a wrong
// password — and never by taking the request goroutine down.
func TestVerifyReadsParametersFromTheHash(t *testing.T) {
	tests := []struct {
		name     string
		password string
		hash     string
		// wantMismatch is a readable hash and the wrong password; wantRefusal
		// is a hash this server will not compute at all.
		wantMismatch bool
		wantRefusal  bool
	}{
		{name: "legacy hash, right password", password: "password", hash: legacyHash},
		{name: "legacy hash, wrong password", password: "wrong", hash: legacyHash, wantMismatch: true},
		{
			name:        "more memory than the ceiling",
			password:    "password",
			hash:        strings.Replace(legacyHash, "m=65536", "m=131072", 1),
			wantRefusal: true,
		},
		{
			name:        "zero memory",
			password:    "password",
			hash:        strings.Replace(legacyHash, "m=65536", "m=0", 1),
			wantRefusal: true,
		},
		{
			name:        "zero passes",
			password:    "password",
			hash:        strings.Replace(legacyHash, "t=1", "t=0", 1),
			wantRefusal: true,
		},
		{
			name:        "zero lanes",
			password:    "password",
			hash:        strings.Replace(legacyHash, "p=4", "p=0", 1),
			wantRefusal: true,
		},
		{
			name:        "more lanes than any hash this server issued",
			password:    "password",
			hash:        strings.Replace(legacyHash, "p=4", "p=64", 1),
			wantRefusal: true,
		},
		{
			name:        "empty key",
			password:    "password",
			hash:        legacyHash[:strings.LastIndex(legacyHash, "$")+1],
			wantRefusal: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := utils.VerifyPassword(t.Context(), tc.password, tc.hash)
			mismatch := errors.Is(err, utils.ErrPasswordMismatch)

			switch {
			case tc.wantRefusal:
				if err == nil || mismatch {
					t.Errorf("VerifyPassword = %v, want a refusal that is not a mismatch", err)
				}
			case tc.wantMismatch:
				if !mismatch {
					t.Errorf("VerifyPassword = %v, want %v", err, utils.ErrPasswordMismatch)
				}
			default:
				if err != nil {
					t.Errorf("VerifyPassword: %v", err)
				}
			}
		})
	}
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
		// 2^22 MiB is 2^32 KiB: it wraps to zero in the uint32 argon2 takes,
		// and argon2 would quietly hash with its minimum instead.
		{name: "memory that wraps to zero in KiB", change: func(p *utils.PasswordHashParams) {
			p.MemoryMiB, p.MaxMemoryMiB = 1<<22, 1<<22
		}},
		{name: "ceiling that wraps in KiB", change: func(p *utils.PasswordHashParams) { p.MaxMemoryMiB = 1<<22 + 32 }},
		{name: "passes that wrap when the stored-hash guard multiplies them", change: func(p *utils.PasswordHashParams) {
			p.Time = 1 << 29
		}},
		{name: "more slots than any host could feed", change: func(p *utils.PasswordHashParams) { p.MaxConcurrent = 1 << 20 }},
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
