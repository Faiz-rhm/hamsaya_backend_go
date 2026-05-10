package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Reject's early-return validation must fire BEFORE any DB call. We
// pass a nil-DB service to prove the check happens up front; otherwise
// notes-less rejections would panic at the Begin/Exec step rather than
// returning a clean validation error to the caller.
func TestDeletionRequestService_RejectRequiresNotes(t *testing.T) {
	svc := &DeletionRequestService{db: nil, logger: zap.NewNop()}
	err := svc.Reject(context.Background(), "deadbeef-id", "admin-id", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejection requires notes")
}

func TestDeletionRequestService_RejectNotesWhitespaceCountsAsEmpty(t *testing.T) {
	// Current implementation only checks for empty string, NOT
	// whitespace. This test documents the existing behaviour so a
	// future tightening (e.g. strings.TrimSpace) is an intentional
	// API change rather than a silent break.
	svc := &DeletionRequestService{db: nil, logger: zap.NewNop()}
	// Whitespace-only notes pass the validation (empty string check).
	// The next call would touch s.db; we recover from the nil-deref to
	// confirm the validation alone didn't return.
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected nil-DB panic, validation accepted whitespace notes")
		}
	}()
	_ = svc.Reject(context.Background(), "id", "admin", "  ")
}

func TestNewDeletionRequestService_ConstructorWires(t *testing.T) {
	svc := NewDeletionRequestService(nil, nil, zap.NewNop())
	require.NotNil(t, svc)
	assert.Nil(t, svc.db)
	assert.Nil(t, svc.adminService)
	assert.NotNil(t, svc.logger)
}
