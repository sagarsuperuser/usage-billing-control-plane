package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// decodeAndValidate decodes JSON from the request body into dst and validates
// struct tags. Returns a user-friendly error message if validation fails.
func decodeAndValidate(r *http.Request, dst any) error {
	if err := decodeJSON(r, dst); err != nil {
		return err
	}
	if err := validate.Struct(dst); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			messages := make([]string, 0, len(validationErrors))
			for _, fe := range validationErrors {
				messages = append(messages, formatValidationError(fe))
			}
			return fmt.Errorf("%s", strings.Join(messages, "; "))
		}
		return err
	}
	return nil
}

func formatValidationError(fe validator.FieldError) string {
	field := toSnakeCase(fe.Field())
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	default:
		return fmt.Sprintf("%s failed validation (%s)", field, fe.Tag())
	}
}

func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+32))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
