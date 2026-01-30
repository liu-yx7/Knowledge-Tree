package store

import "context"

// ContentType represents the type of content being synced.
type ContentType string

const (
	ContentTypeMemo       ContentType = "memo"
	ContentTypeAttachment ContentType = "attachment"
)

// RAGFlowSyncStatus represents the sync status of content with RAGFlow.
type RAGFlowSyncStatus string

const (
	RAGFlowSyncStatusPending RAGFlowSyncStatus = "pending"
	RAGFlowSyncStatusSynced  RAGFlowSyncStatus = "synced"
	RAGFlowSyncStatusFailed  RAGFlowSyncStatus = "failed"
	RAGFlowSyncStatusSkipped RAGFlowSyncStatus = "skipped"
)

// ContentSyncState represents the RAGFlow sync state of a memo or attachment.
type ContentSyncState struct {
	ID                int32
	ContentType       ContentType
	ContentUID        string
	OwnerID           int32
	RAGFlowStatus     RAGFlowSyncStatus
	RAGFlowDatasetID  string
	RAGFlowDocumentID string
	RAGFlowSyncedTs   *int64
	RAGFlowError      string
	ContentHash       string
	RetryCount        int32
	NextRetryTs       *int64
	CreatedTs         int64
	UpdatedTs         int64
}

// FindContentSyncState specifies filter criteria for finding sync states.
type FindContentSyncState struct {
	ID            *int32
	ContentType   *ContentType
	ContentUID    *string
	OwnerID       *int32
	RAGFlowStatus *RAGFlowSyncStatus
	Limit         *int
	Offset        *int
}

// UpdateContentSyncState specifies fields to update.
type UpdateContentSyncState struct {
	ID                int32
	RAGFlowStatus     *RAGFlowSyncStatus
	RAGFlowDatasetID  *string
	RAGFlowDocumentID *string
	RAGFlowSyncedTs   *int64
	RAGFlowError      *string
	ContentHash       *string
	RetryCount        *int32
	NextRetryTs       *int64
	UpdatedTs         *int64
}

// DeleteContentSyncState specifies which sync state to delete.
type DeleteContentSyncState struct {
	ID          *int32
	ContentType *ContentType
	ContentUID  *string
	OwnerID     *int32
}

// CreateContentSyncState creates a new content sync state.
func (s *Store) CreateContentSyncState(ctx context.Context, create *ContentSyncState) (*ContentSyncState, error) {
	return s.driver.CreateContentSyncState(ctx, create)
}

// GetContentSyncState returns a single sync state matching the filter.
func (s *Store) GetContentSyncState(ctx context.Context, find *FindContentSyncState) (*ContentSyncState, error) {
	list, err := s.driver.ListContentSyncStates(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

// ListContentSyncStates returns sync states matching the filter.
func (s *Store) ListContentSyncStates(ctx context.Context, find *FindContentSyncState) ([]*ContentSyncState, error) {
	return s.driver.ListContentSyncStates(ctx, find)
}

// UpdateContentSyncState updates a sync state.
func (s *Store) UpdateContentSyncState(ctx context.Context, update *UpdateContentSyncState) error {
	return s.driver.UpdateContentSyncState(ctx, update)
}

// DeleteContentSyncState deletes sync states matching the filter.
func (s *Store) DeleteContentSyncState(ctx context.Context, delete *DeleteContentSyncState) error {
	return s.driver.DeleteContentSyncState(ctx, delete)
}

// UpsertContentSyncState creates or updates a content sync state.
func (s *Store) UpsertContentSyncState(ctx context.Context, create *ContentSyncState) (*ContentSyncState, error) {
	return s.driver.UpsertContentSyncState(ctx, create)
}
