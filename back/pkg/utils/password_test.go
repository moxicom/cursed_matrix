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
			hash, err := utils.HashPassword(tc.password)
			if err != nil {
				t.Fatalf("HashPassword: %v", err)
			}
			if strings.Contains(hash, tc.password) {
				t.Fatal("the hash contains the password")
			}

			err = utils.VerifyPassword(tc.verify, hash)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyPassword = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestHashIsSaltedPerCall(t *testing.T) {
	first, err := utils.HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	second, err := utils.HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if first == second {
		t.Fatal("two hashes of one password are identical; the salt is not random")
	}
	for _, hash := range []string{first, second} {
		if err := utils.VerifyPassword("same password", hash); err != nil {
			t.Errorf("a freshly made hash does not verify: %v", err)
		}
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	valid, err := utils.HashPassword("password")
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
		{name: "unreadable parameters", hash: strings.Replace(valid, "m=65536", "m=many", 1)},
		{name: "corrupt salt", hash: strings.Replace(valid, "$argon2id$", "$argon2id$!", 1)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := utils.VerifyPassword("password", tc.hash); err == nil {
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
			if err := utils.VerifyAbsentAccount(tc.password); !errors.Is(err, utils.ErrPasswordMismatch) {
				t.Errorf("VerifyAbsentAccount = %v, want ErrPasswordMismatch", err)
			}
		})
	}
}
