// Package tag holds the user's own labels for tasks.
package tag

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// NameMaxLen matches the CHECK on the column.
const NameMaxLen = 24

// Tag is one label. TaskCount is a read-side figure: the client's collapsing
// tag filter decides what to show from it, so it comes from the server rather
// than being counted in the browser over a page of tasks it may not have.
type Tag struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	TaskCount int
}

// NormalizeName trims a name and enforces its length in runes.
//
// Case is kept as the user typed it — the uniqueness rule folds case, so
// "Backend" and "backend" are one tag, and the first spelling is the one
// everybody then sees.
func NormalizeName(name string) (string, error) {
	clean := strings.TrimSpace(name)

	switch {
	case clean == "":
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "name", "reason": "empty"})
	case utf8.RuneCountInString(clean) > NameMaxLen:
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "name", "max": NameMaxLen})
	}
	return clean, nil
}
