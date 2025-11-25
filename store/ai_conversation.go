package store

import (
	"context"
)

type AIConversation struct {
	ID           int32
	Name         string
	CreatorID    int32
	LLMProvider  string
	LLMModel     string
	SystemPrompt string
	CreatedTs    int64
	UpdatedTs    int64
}

type FindAIConversation struct {
	ID        *int32
	CreatorID *int32
	// Pagination
	Limit  *int
	Offset *int
}

type UpdateAIConversation struct {
	ID           int32
	Name         *string
	SystemPrompt *string
	UpdatedTs    *int64
}

type DeleteAIConversation struct {
	ID int32
}

func (s *Store) CreateAIConversation(ctx context.Context, create *AIConversation) (*AIConversation, error) {
	return s.driver.CreateAIConversation(ctx, create)
}

func (s *Store) ListAIConversations(ctx context.Context, find *FindAIConversation) ([]*AIConversation, error) {
	return s.driver.ListAIConversations(ctx, find)
}

func (s *Store) GetAIConversation(ctx context.Context, find *FindAIConversation) (*AIConversation, error) {
	list, err := s.ListAIConversations(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (s *Store) UpdateAIConversation(ctx context.Context, update *UpdateAIConversation) error {
	return s.driver.UpdateAIConversation(ctx, update)
}

func (s *Store) DeleteAIConversation(ctx context.Context, delete *DeleteAIConversation) error {
	return s.driver.DeleteAIConversation(ctx, delete)
}
