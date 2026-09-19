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
)

func parseEnums(t *testing.T) map[string][]string {
	t.Helper()

	raw, err := migrations.FS.ReadFile("20260919120000_enum_types.sql")
	if err != nil {
		t.Fatalf("read the enum migration: %v", err)
	}

	sql := string(raw)
	if idx := strings.Index(sql, "-- +goose Down"); idx > 0 {
		sql = sql[:idx]
	}

	out := make(map[string][]string)
	for _, block := range enumBlock.FindAllStringSubmatch(sql, -1) {
		var values []string
		for _, match := range enumValue.FindAllStringSubmatch(block[2], -1) {
			values = append(values, match[1])
		}
		out[block[1]] = values
	}
	return out
}
