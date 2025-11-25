package store

import (
	"context"
)

type AIProviderConfig struct {
	ID          int32
	Name        string
	DisplayName string
	APIKey      string
	APIEndpoint string
	Config      string // JSON
	Enabled     bool
	CreatedTs   int64
	UpdatedTs   int64
}

type FindAIProviderConfig struct {
	ID      *int32
	Name    *string
	Enabled *bool
}

type UpdateAIProviderConfig struct {
	ID          int32
	DisplayName *string
	APIKey      *string
	APIEndpoint *string
	Config      *string
	Enabled     *bool
	UpdatedTs   *int64
}

type DeleteAIProviderConfig struct {
	ID int32
}

func (s *Store) CreateAIProviderConfig(ctx context.Context, create *AIProviderConfig) (*AIProviderConfig, error) {
	return s.driver.CreateAIProviderConfig(ctx, create)
}

func (s *Store) ListAIProviderConfigs(ctx context.Context, find *FindAIProviderConfig) ([]*AIProviderConfig, error) {
	return s.driver.ListAIProviderConfigs(ctx, find)
}

func (s *Store) GetAIProviderConfig(ctx context.Context, find *FindAIProviderConfig) (*AIProviderConfig, error) {
	list, err := s.ListAIProviderConfigs(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (s *Store) UpdateAIProviderConfig(ctx context.Context, update *UpdateAIProviderConfig) error {
	return s.driver.UpdateAIProviderConfig(ctx, update)
}

func (s *Store) DeleteAIProviderConfig(ctx context.Context, delete *DeleteAIProviderConfig) error {
	return s.driver.DeleteAIProviderConfig(ctx, delete)
}
