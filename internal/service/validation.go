package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var serviceValidator = validator.New(validator.WithRequiredStructEnabled())

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func init() {
	_ = serviceValidator.RegisterValidation("future_time", validateFutureTime)
}

func validateInput(input any) error {
	if err := serviceValidator.Struct(input); err != nil {
		validationErrors := TranslateValidationErrors(err)
		if len(validationErrors) > 0 {
			return newValidationError(validationErrors)
		}

		return ErrInvalidInput
	}

	return nil
}

type inputValidationError struct {
	errors []ValidationError
}

func (e inputValidationError) Error() string {
	parts := make([]string, 0, len(e.errors))
	for _, validationErr := range e.errors {
		parts = append(parts, validationErr.Message)
	}

	return fmt.Sprintf("invalid input: %s", strings.Join(parts, "; "))
}

func (e inputValidationError) Unwrap() error {
	return ErrInvalidInput
}

func newValidationError(errors []ValidationError) error {
	return inputValidationError{errors: errors}
}

func translateError(fe validator.FieldError) ValidationError {
	field := strings.ToLower(fe.Field())

	messages := map[string]string{
		"required":    fmt.Sprintf("%s is required", field),
		"email":       fmt.Sprintf("%s must be a valid email address", field),
		"min":         fmt.Sprintf("%s must be at least %s characters", field, fe.Param()),
		"max":         fmt.Sprintf("%s must not exceed %s characters", field, fe.Param()),
		"gte":         fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param()),
		"lte":         fmt.Sprintf("%s must be less than or equal to %s", field, fe.Param()),
		"gt":          fmt.Sprintf("%s must be greater than %s", field, fe.Param()),
		"lt":          fmt.Sprintf("%s must be less than %s", field, fe.Param()),
		"eqfield":     fmt.Sprintf("%s must match %s", field, fe.Param()),
		"oneof":       fmt.Sprintf("%s must be one of: %s", field, fe.Param()),
		"url":         fmt.Sprintf("%s must be a valid URL", field),
		"uuid4":       fmt.Sprintf("%s must be a valid UUID", field),
		"alphanum":    fmt.Sprintf("%s must contain only alphanumeric characters", field),
		"numeric":     fmt.Sprintf("%s must be a number", field),
		"future_time": fmt.Sprintf("%s must be in the future", field),
	}

	if message, ok := messages[fe.Tag()]; ok {
		return ValidationError{Field: field, Message: message}
	}

	return ValidationError{Field: field, Message: fmt.Sprintf("%s failed validation on %s", field, fe.Tag())}
}

func TranslateValidationErrors(err error) []ValidationError {
	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	if !ok {
		return nil
	}

	errs := make([]ValidationError, 0, len(validationErrors))
	for _, validationErr := range validationErrors {
		errs = append(errs, translateError(validationErr))
	}

	return errs
}

func validateFutureTime(fl validator.FieldLevel) bool {
	value, ok := fl.Field().Interface().(time.Time)
	if !ok {
		return false
	}

	return value.After(time.Now().UTC())
}
