package store

import "context"

// Notebook represents a collection of memos, mapping to an independent RAGFlow Dataset.
type Notebook struct {
	ID        int32
	UID       string
	CreatorID int32
	Title     string
	Icon      string
	IsDefault bool

	// RAGFlow Dataset binding
	DatasetID string

	// Standard fields
	RowStatus RowStatus
	CreatedTs int64
	UpdatedTs int64
}

// FindNotebook specifies the conditions to find notebooks.
type FindNotebook struct {
	ID        *int32
	UID       *string
	CreatorID *int32
	IsDefault *bool
	RowStatus *RowStatus
}

// UpdateNotebook specifies the fields to update in a notebook.
type UpdateNotebook struct {
	ID        int32
	Title     *string
	Icon      *string
	DatasetID *string
	RowStatus *RowStatus
}

// DeleteNotebook specifies the conditions to delete a notebook.
type DeleteNotebook struct {
	ID int32
}

func (s *Store) CreateNotebook(ctx context.Context, create *Notebook) (*Notebook, error) {
	return s.driver.CreateNotebook(ctx, create)
}

func (s *Store) ListNotebooks(ctx context.Context, find *FindNotebook) ([]*Notebook, error) {
	return s.driver.ListNotebooks(ctx, find)
}

func (s *Store) GetNotebook(ctx context.Context, find *FindNotebook) (*Notebook, error) {
	list, err := s.ListNotebooks(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (s *Store) UpdateNotebook(ctx context.Context, update *UpdateNotebook) error {
	return s.driver.UpdateNotebook(ctx, update)
}

func (s *Store) DeleteNotebook(ctx context.Context, delete *DeleteNotebook) error {
	return s.driver.DeleteNotebook(ctx, delete)
}
