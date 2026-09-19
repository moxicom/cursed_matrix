package postgres

import (
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

var builder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var sortColumns = map[task.SortField]string{
	task.SortByPosition: "t.position",
	task.SortByDeadline: "t.deadline_at",
	task.SortByCreated:  "t.created_at",
	task.SortByTitle:    "lower(t.title)",
}

var sortDirections = map[task.SortDirection]string{
	task.SortAsc:  "ASC",
	task.SortDesc: "DESC",
}

func orderClause(field task.SortField, direction task.SortDirection) (string, error) {
	column, ok := sortColumns[field]
	if !ok {
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "sort", "value": string(field)})
	}
	dir, ok := sortDirections[direction]
	if !ok {
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "direction", "value": string(direction)})
	}
	return column + " " + dir + " NULLS LAST", nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func containsPattern(term string) string {
	return "%" + likeEscaper.Replace(term) + "%"
}

func localDayBounds(instant time.Time, loc *time.Location) (time.Time, time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	local := instant.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1)
}

func enumStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}
