package utils

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAppError(t *testing.T) {
	wrappedErr := errors.New("underlying cause")
	appErr := NewAppError(http.StatusBadRequest, "something went wrong", wrappedErr)

	assert.Equal(t, http.StatusBadRequest, appErr.Code)
	assert.Equal(t, "something went wrong", appErr.Message)
	assert.Equal(t, wrappedErr, appErr.Err)
}

func TestAppError_Error_WithWrappedErr(t *testing.T) {
	wrappedErr := errors.New("wrapped error text")
	appErr := NewAppError(http.StatusInternalServerError, "message", wrappedErr)

	// Error() returns "message: cause" when Err is set
	assert.Equal(t, "message: wrapped error text", appErr.Error())
}

func TestAppError_Error_WithoutWrappedErr(t *testing.T) {
	appErr := NewAppError(http.StatusBadRequest, "just the message", nil)

	// When Err is nil, Error() returns Message
	assert.Equal(t, "just the message", appErr.Error())
}

func TestNewBadRequestError(t *testing.T) {
	err := NewBadRequestError("bad input", nil)

	assert.Equal(t, http.StatusBadRequest, err.Code)
	assert.Equal(t, "bad input", err.Message)
	assert.Nil(t, err.Err)
}

func TestNewBadRequestError_WithWrapped(t *testing.T) {
	cause := errors.New("parse failure")
	err := NewBadRequestError("bad input", cause)

	assert.Equal(t, http.StatusBadRequest, err.Code)
	assert.Equal(t, cause, err.Err)
}

func TestNewUnauthorizedError(t *testing.T) {
	err := NewUnauthorizedError("not authorized", nil)

	assert.Equal(t, http.StatusUnauthorized, err.Code)
	assert.Equal(t, "not authorized", err.Message)
}

func TestNewForbiddenError(t *testing.T) {
	err := NewForbiddenError("forbidden action", nil)

	assert.Equal(t, http.StatusForbidden, err.Code)
	assert.Equal(t, "forbidden action", err.Message)
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("resource not found", nil)

	assert.Equal(t, http.StatusNotFound, err.Code)
	assert.Equal(t, "resource not found", err.Message)
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("already exists", nil)

	assert.Equal(t, http.StatusConflict, err.Code)
	assert.Equal(t, "already exists", err.Message)
}

func TestNewInternalServerError(t *testing.T) {
	cause := errors.New("db timeout")
	err := NewInternalServerError("internal failure", cause)

	assert.Equal(t, http.StatusInternalServerError, err.Code)
	assert.Equal(t, "internal failure", err.Message)
	assert.Equal(t, cause, err.Err)
}

func TestNewInternalError_IsAliasForInternalServerError(t *testing.T) {
	cause := errors.New("some db error")
	err := NewInternalError("internal error", cause)

	assert.Equal(t, http.StatusInternalServerError, err.Code)
	assert.Equal(t, "internal error", err.Message)
	assert.Equal(t, cause, err.Err)
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("validation failed", nil)

	assert.Equal(t, http.StatusUnprocessableEntity, err.Code)
	assert.Equal(t, "validation failed", err.Message)
}

func TestNewNotImplementedError(t *testing.T) {
	err := NewNotImplementedError("not yet implemented", nil)

	assert.Equal(t, http.StatusNotImplemented, err.Code)
	assert.Equal(t, "not yet implemented", err.Message)
}

func TestAppError_WrappingBehavior(t *testing.T) {
	cause := errors.New("root cause")
	appErr := NewInternalServerError("outer message", cause)

	// The wrapped error should be accessible via errors.Is / errors.As on the Err field
	assert.True(t, errors.Is(appErr.Err, cause))

	// Error() returns "message: cause" when Err is non-nil
	assert.Equal(t, "outer message: root cause", appErr.Error())
}

func TestCommonErrorSentinels(t *testing.T) {
	// Verify sentinel variables are non-nil and have expected messages
	sentinels := map[string]error{
		"internal server error":   ErrInternalServer,
		"bad request":             ErrBadRequest,
		"unauthorized":            ErrUnauthorized,
		"forbidden":               ErrForbidden,
		"not found":               ErrNotFound,
		"conflict":                ErrConflict,
		"validation error":        ErrValidation,
		"invalid JSON":            ErrInvalidJSON,
		"invalid token":           ErrInvalidToken,
		"expired token":           ErrExpiredToken,
		"user already exists":     ErrUserExists,
		"user not found":          ErrUserNotFound,
		"invalid credentials":     ErrInvalidCredentials,
		"account locked":          ErrAccountLocked,
		"email not verified":      ErrEmailNotVerified,
		"MFA verification required": ErrMFARequired,
	}

	for expectedMsg, sentinel := range sentinels {
		assert.NotNil(t, sentinel, "sentinel should not be nil: %s", expectedMsg)
		assert.Equal(t, expectedMsg, sentinel.Error(), "sentinel message mismatch")
	}
}

func TestAppError_NilErrUsesMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		err     error
		wantMsg string
	}{
		{
			name:    "nil wrapped err returns message",
			message: "descriptive message",
			err:     nil,
			wantMsg: "descriptive message",
		},
		{
			name:    "non-nil wrapped err returns message: cause",
			message: "descriptive message",
			err:     errors.New("inner error"),
			wantMsg: "descriptive message: inner error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := NewAppError(http.StatusBadRequest, tt.message, tt.err)
			assert.Equal(t, tt.wantMsg, appErr.Error())
		})
	}
}
