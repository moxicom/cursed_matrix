package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var configValidator = newConfigValidator()

func newConfigValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})
	return v
}

func validateStructure(cfg *Config) error {
	err := configValidator.Struct(cfg)
	if err == nil {
		return nil
	}

	var failures validator.ValidationErrors
	if !errors.As(err, &failures) {
		return err
	}

	reasons := make([]string, 0, len(failures))
	for _, failure := range failures {
		reasons = append(reasons, describe(failure))
	}
	return fmt.Errorf("invalid values: %s", strings.Join(reasons, "; "))
}

var yamlNames = map[string]string{
	"MaxConns":  "max_conns",
	"AccessTTL": "access_ttl",
}

func describe(failure validator.FieldError) string {
	field := strings.TrimPrefix(failure.Namespace(), "Config.")

	param := failure.Param()
	if spelled, ok := yamlNames[param]; ok {
		param = spelled
	}

	switch failure.Tag() {
	case "required":
		return field + " is required"
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s], got %v", field, param, failure.Value())
	case "gt", "gte", "min":
		return fmt.Sprintf("%s must be at least %s, got %v", field, param, failure.Value())
	case "max":
		return fmt.Sprintf("%s must be at most %s, got %v", field, param, failure.Value())
	case "ltefield":
		return fmt.Sprintf("%s must not exceed %s, got %v", field, param, failure.Value())
	case "gtfield":
		return fmt.Sprintf("%s must be greater than %s, got %v", field, param, failure.Value())
	default:
		return fmt.Sprintf("%s fails %s, got %v", field, failure.Tag(), failure.Value())
	}
}
