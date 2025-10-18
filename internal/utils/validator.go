package utils

import (
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

// Validator wraps the go-playground validator
type Validator struct {
	validator *validator.Validate
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		validator: validator.New(),
	}
}

// Validate validates a struct
func (v *Validator) Validate(s interface{}) error {
	if err := v.validator.Struct(s); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			// Return first error with formatted message
			if len(validationErrors) > 0 {
				return FormatValidationError(validationErrors[0])
			}
		}
		return err
	}
	return nil
}

// InitValidator initializes the validator
func InitValidator() {
	validate = validator.New()
}

// GetValidator returns the validator instance
func GetValidator() *validator.Validate {
	if validate == nil {
		InitValidator()
	}
	return validate
}

// ValidateStruct validates a struct
func ValidateStruct(s interface{}) error {
	return GetValidator().Struct(s)
}

// ValidateVar validates a single variable
func ValidateVar(field interface{}, tag string) error {
	return GetValidator().Var(field, tag)
}

// FormatValidationError formats a single validation error into a readable message
func FormatValidationError(fieldError validator.FieldError) error {
	field := fieldError.Field()
	tag := fieldError.Tag()

	var message string
	switch tag {
	case "required":
		message = field + " is required"
	case "email":
		message = field + " must be a valid email address"
	case "min":
		message = field + " must be at least " + fieldError.Param() + " characters"
	case "max":
		message = field + " must be at most " + fieldError.Param() + " characters"
	case "gte":
		message = field + " must be greater than or equal to " + fieldError.Param()
	case "lte":
		message = field + " must be less than or equal to " + fieldError.Param()
	case "len":
		message = field + " must be exactly " + fieldError.Param() + " characters"
	case "oneof":
		message = field + " must be one of: " + fieldError.Param()
	default:
		message = field + " is invalid"
	}

	return NewValidationError(message, fieldError)
}

// FormatValidationErrors formats validation errors into a readable map
func FormatValidationErrors(err error) map[string]string {
	errors := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErrors {
			field := fieldError.Field()
			tag := fieldError.Tag()

			switch tag {
			case "required":
				errors[field] = field + " is required"
			case "email":
				errors[field] = field + " must be a valid email address"
			case "min":
				errors[field] = field + " must be at least " + fieldError.Param() + " characters"
			case "max":
				errors[field] = field + " must be at most " + fieldError.Param() + " characters"
			case "gte":
				errors[field] = field + " must be greater than or equal to " + fieldError.Param()
			case "lte":
				errors[field] = field + " must be less than or equal to " + fieldError.Param()
			default:
				errors[field] = field + " is invalid"
			}
		}
	}

	return errors
}
