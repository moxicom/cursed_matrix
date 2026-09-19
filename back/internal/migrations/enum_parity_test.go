package migrations_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/migrations"
)

func TestGoEnumsMatchTheMigration(t *testing.T) {
	declared := parseEnums(t)
	inGo := shared.PGEnumTypes()

	names := make([]string, 0, len(inGo))
	for name := range inGo {
		names = append(names, name)
	}
	for name := range declared {
		if _, known := inGo[name]; !known {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			want, inGoOK := inGo[name]
			got, inSQLOK := declared[name]

			switch {
			case !inSQLOK:
				t.Fatalf("type %s exists in Go but no migration creates it", name)
			case !inGoOK:
				t.Fatalf("type %s is created by a migration but is unknown to Go", name)
			case !slices.Equal(got, want):
				t.Fatalf("values differ\n  migration: %v\n  go:        %v", got, want)
			}
		})
	}
}

var (
	enumBlock = regexp.MustCompile(`(?is)CREATE\s+TYPE\s+(\w+)\s+AS\s+ENUM\s*\(([^)]*)\)`)
	enumValue = regexp.MustCompile(`'([^']*)'`)

	enumAddValue = regexp.MustCompile(`(?is)ALTER\s+TYPE\s+(\w+)\s+ADD\s+VALUE(?:\s+IF\s+NOT\s+EXISTS)?\s+'([^']*)'`)
)

// parseEnums replays every migration's enum statements in order, so a value
// added by a later ALTER TYPE counts as declared. Reading only the file that
// creates the types would let Go and the database drift apart the moment a
// value is added.
func parseEnums(t *testing.T) map[string][]string {
	t.Helper()

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read the migrations: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)

	out := make(map[string][]string)
	for _, name := range names {
		raw, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		sql := string(raw)
		if idx := strings.Index(sql, "-- +goose Down"); idx > 0 {
			sql = sql[:idx]
		}

		for _, block := range enumBlock.FindAllStringSubmatch(sql, -1) {
			var values []string
			for _, match := range enumValue.FindAllStringSubmatch(block[2], -1) {
				values = append(values, match[1])
			}
			out[block[1]] = values
		}
		for _, added := range enumAddValue.FindAllStringSubmatch(sql, -1) {
			out[added[1]] = append(out[added[1]], added[2])
		}
	}
	return out
}
