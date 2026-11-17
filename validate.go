package deepseek

import "fmt"

// ValidationError reports a request that violates a documented constraint.
// It is returned before any network call.
type ValidationError struct {
	Field   string
	Problem string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return "deepseek: invalid request: " + e.Problem
	}
	return fmt.Sprintf("deepseek: invalid %s: %s", e.Field, e.Problem)
}

// Invalid builds a ValidationError.
func Invalid(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Problem: fmt.Sprintf(format, args...)}
}
