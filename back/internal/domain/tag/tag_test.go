package tag_test

import (
	"strings"
	"testing"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/tag"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "an ordinary name", input: "backend", want: "backend"},
		{name: "surrounding space is trimmed", input: "  backend  ", want: "backend"},
		{name: "case is the user's to choose", input: "Backend", want: "Backend"},
		{name: "empty is refused", input: "", wantErr: true},
		{name: "only space is refused", input: "   ", wantErr: true},
		{name: "exactly the limit fits", input: strings.Repeat("a", 24), want: strings.Repeat("a", 24)},
		{name: "one over the limit is refused", input: strings.Repeat("a", 25), wantErr: true},
		{
			// The limit is in runes, not bytes: 24 emoji are 24 characters to
			// the user and 96 bytes to the column.
			name:  "a name of multi-byte runes is measured in runes",
			input: strings.Repeat("я", 24),
			want:  strings.Repeat("я", 24),
		},
		{name: "multi-byte runes past the limit are refused", input: strings.Repeat("я", 25), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tag.NormalizeName(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeName(%q) = %q, want an error", tt.input, got)
				}
				if shared.CodeOf(err) != shared.CodeValidationFailed {
					t.Errorf("code = %s", shared.CodeOf(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeName(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("= %q, want %q", got, tt.want)
			}
		})
	}
}
