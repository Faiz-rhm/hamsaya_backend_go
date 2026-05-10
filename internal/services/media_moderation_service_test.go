package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Reject must require notes — the validation must run BEFORE a DB
// transaction is opened so notes-less callers get a 400 instead of a
// half-rolled-back tx.
func TestMediaModerationService_RejectRequiresNotes(t *testing.T) {
	svc := &MediaModerationService{db: nil, logger: zap.NewNop()}
	err := svc.Reject(context.Background(), "att-id", "admin-id", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejection requires notes")
}

func TestNewMediaModerationService_ConstructorWires(t *testing.T) {
	svc := NewMediaModerationService(nil, zap.NewNop())
	require.NotNil(t, svc)
	assert.Nil(t, svc.db)
	assert.NotNil(t, svc.logger)
}
