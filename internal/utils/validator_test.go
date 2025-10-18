package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	Email    string `validate:"required,email"`
	Password string `validate:"required,min=8"`
	Age      int    `validate:"gte=0,lte=150"`
}

func TestValidateStruct_Valid(t *testing.T) {
	InitValidator()

	validData := TestStruct{
		Email:    "test@example.com",
		Password: "password123",
		Age:      25,
	}

	err := ValidateStruct(validData)
	assert.NoError(t, err)
}

func TestValidateStruct_Invalid(t *testing.T) {
	InitValidator()

	invalidData := TestStruct{
		Email:    "invalid-email",
		Password: "short",
		Age:      -1,
	}

	err := ValidateStruct(invalidData)
	assert.Error(t, err)
}

func TestFormatValidationErrors(t *testing.T) {
	InitValidator()

	invalidData := TestStruct{
		Email:    "invalid-email",
		Password: "short",
		Age:      200,
	}

	err := ValidateStruct(invalidData)
	assert.Error(t, err)

	errors := FormatValidationErrors(err)
	assert.NotEmpty(t, errors)
	assert.Contains(t, errors, "Email")
	assert.Contains(t, errors, "Password")
	assert.Contains(t, errors, "Age")
}
