package store

import (
	"context"
)

type AIMessage struct {
	ID             int32
	ConversationID int32
	Role           string // "user" or "assistant"
	Content        string
	Tokens         int32
	CreatedTs      int64
}

type FindAIMessage struct {
	ID             *int32
	ConversationID *int32
	// Pagination
	Limit  *int
	Offset *int
}

type DeleteAIMessage struct {
	ID int32
}

func (s *Store) CreateAIMessage(ctx context.Context, create *AIMessage) (*AIMessage, error) {
	return s.driver.CreateAIMessage(ctx, create)
}

func (s *Store) ListAIMessages(ctx context.Context, find *FindAIMessage) ([]*AIMessage, error) {
	return s.driver.ListAIMessages(ctx, find)
}

func (s *Store) GetAIMessage(ctx context.Context, find *FindAIMessage) (*AIMessage, error) {
	list, err := s.ListAIMessages(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (s *Store) DeleteAIMessage(ctx context.Context, delete *DeleteAIMessage) error {
	return s.driver.DeleteAIMessage(ctx, delete)
}
