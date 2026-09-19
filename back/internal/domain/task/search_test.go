package task_test

import (
	"strings"
	"testing"

	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

func TestDescribeMatch(t *testing.T) {
	tagName := "backend"

	tests := []struct {
		name        string
		title       string
		description string
		tag         *string
		term        string
		wantField   task.Match
		wantText    string
	}{
		{
			name:      "the title wins",
			title:     "Patch the auth refresh",
			term:      "auth",
			wantField: task.MatchTitle,
			wantText:  "Patch the auth refresh",
		},
		{
			name:        "the description when the title says nothing",
			title:       "Patch it",
			description: "the refresh token races with the cookie",
			term:        "refresh",
			wantField:   task.MatchDescription,
			wantText:    "the refresh token races with the cookie",
		},
		{
			name:      "a tag when neither text matches",
			title:     "Patch it",
			tag:       &tagName,
			term:      "backend",
			wantField: task.MatchTag,
			wantText:  "#backend",
		},
		{
			name:      "matching is case-insensitive",
			title:     "Patch the AUTH refresh",
			term:      "auth",
			wantField: task.MatchTitle,
		},
		{
			// The row came back, so something matched; naming a field that did
			// not would be a lie the client renders.
			name:      "nothing identifiable falls back to the title",
			title:     "Patch it",
			term:      "мимо",
			wantField: task.MatchTitle,
			wantText:  "Patch it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, text := task.DescribeMatch(tt.title, tt.description, tt.tag, tt.term)

			if field != tt.wantField {
				t.Errorf("field = %s, want %s", field, tt.wantField)
			}
			if tt.wantText != "" && text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}

func TestSnippet(t *testing.T) {
	long := strings.Repeat("a", 100) + "NEEDLE" + strings.Repeat("b", 100)

	tests := []struct {
		name      string
		text      string
		term      string
		wantHas   string
		wantEdges [2]bool
		wantWhole bool
		wantRunes int
	}{
		{
			name:      "a short description comes whole",
			text:      "the refresh token races",
			term:      "refresh",
			wantWhole: true,
		},
		{
			name:      "a match in the middle is cut on both sides",
			text:      long,
			term:      "NEEDLE",
			wantHas:   "NEEDLE",
			wantEdges: [2]bool{true, true},
		},
		{
			name:      "a term that is not there still yields a lead",
			text:      long,
			term:      "absent",
			wantEdges: [2]bool{false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := task.Snippet(tt.text, tt.term)

			if tt.wantWhole && got != tt.text {
				t.Fatalf("= %q, want the whole text", got)
			}
			if tt.wantHas != "" && !strings.Contains(got, tt.wantHas) {
				t.Errorf("= %q, want it to contain %q", got, tt.wantHas)
			}
			if !tt.wantWhole {
				if strings.HasPrefix(got, "…") != tt.wantEdges[0] {
					t.Errorf("leading ellipsis = %v, want %v (%q)", !tt.wantEdges[0], tt.wantEdges[0], got)
				}
				if strings.HasSuffix(got, "…") != tt.wantEdges[1] {
					t.Errorf("trailing ellipsis = %v, want %v (%q)", !tt.wantEdges[1], tt.wantEdges[1], got)
				}
			}
		})
	}
}

// TestSnippetCutsWholeRunes is why the cut counts runes: slicing bytes in the
// middle of a multi-byte character produces text no client can render.
func TestSnippetCutsWholeRunes(t *testing.T) {
	text := strings.Repeat("я", 80) + "иголка" + strings.Repeat("ю", 80)

	got := task.Snippet(text, "иголка")

	if !strings.Contains(got, "иголка") {
		t.Fatalf("= %q, want the term kept", got)
	}
	if !utf8ValidString(got) {
		t.Errorf("= %q, which is not valid UTF-8", got)
	}
}

func utf8ValidString(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}
