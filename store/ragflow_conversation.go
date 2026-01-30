package store

import "context"

// RAGFlowConversation represents a chat conversation with RAGFlow.
type RAGFlowConversation struct {
	ID               int32
	UID              string
	UserID           int32
	RAGFlowSessionID string
	Title            string
	CreatedTs        int64
	UpdatedTs        int64
	RowStatus        RowStatus
}

// FindRAGFlowConversation specifies filter criteria for finding conversations.
type FindRAGFlowConversation struct {
	ID               *int32
	UID              *string
	UserID           *int32
	RAGFlowSessionID *string
	RowStatus        *RowStatus
	Limit            *int
	Offset           *int
}

// UpdateRAGFlowConversation specifies fields to update.
type UpdateRAGFlowConversation struct {
	ID        int32
	Title     *string
	RowStatus *RowStatus
	UpdatedTs *int64
}

// DeleteRAGFlowConversation specifies which conversation to delete.
type DeleteRAGFlowConversation struct {
	ID     *int32
	UserID *int32
}

// CreateRAGFlowConversation creates a new RAGFlow conversation.
func (s *Store) CreateRAGFlowConversation(ctx context.Context, create *RAGFlowConversation) (*RAGFlowConversation, error) {
	return s.driver.CreateRAGFlowConversation(ctx, create)
}

// GetRAGFlowConversation returns a single conversation matching the filter.
func (s *Store) GetRAGFlowConversation(ctx context.Context, find *FindRAGFlowConversation) (*RAGFlowConversation, error) {
	list, err := s.driver.ListRAGFlowConversations(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

// ListRAGFlowConversations returns conversations matching the filter.
func (s *Store) ListRAGFlowConversations(ctx context.Context, find *FindRAGFlowConversation) ([]*RAGFlowConversation, error) {
	return s.driver.ListRAGFlowConversations(ctx, find)
}

// UpdateRAGFlowConversation updates a conversation.
func (s *Store) UpdateRAGFlowConversation(ctx context.Context, update *UpdateRAGFlowConversation) error {
	return s.driver.UpdateRAGFlowConversation(ctx, update)
}

// DeleteRAGFlowConversation deletes conversations matching the filter.
func (s *Store) DeleteRAGFlowConversation(ctx context.Context, delete *DeleteRAGFlowConversation) error {
	return s.driver.DeleteRAGFlowConversation(ctx, delete)
}
