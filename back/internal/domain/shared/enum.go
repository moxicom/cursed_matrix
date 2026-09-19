package shared

import (
	"database/sql/driver"
	"fmt"
	"slices"
)

func encodeEnum[T ~string](v *T, valid func(*T) bool, name string) (driver.Value, error) {
	if !valid(v) {
		return nil, fmt.Errorf("%s: invalid value %q", name, string(*v))
	}
	return string(*v), nil
}

func decodeEnum[T ~string](dst *T, src any, valid func(*T) bool, name string) error {
	var s string
	switch v := src.(type) {
	case nil:
		return fmt.Errorf("%s: cannot scan NULL into a non-pointer enum", name)
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("%s: cannot scan %T", name, src)
	}
	candidate := T(s)
	if !valid(&candidate) {
		return fmt.Errorf("%s: unknown value %q", name, s)
	}
	*dst = candidate
	return nil
}

func parseEnum[T ~string](s string, valid func(*T) bool, field string) (T, error) {
	v := T(s)
	if !valid(&v) {
		return T(""), NewError(CodeValidationFailed, map[string]any{"field": field, "value": s})
	}
	return v, nil
}

func contains[T comparable](set []T, v T) bool {
	return slices.Contains(set, v)
}

func codes[T ~string](set []T) []string {
	out := make([]string, len(set))
	for i, v := range set {
		out[i] = string(v)
	}
	return out
}
