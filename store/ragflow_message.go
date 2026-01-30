package store

import "context"

// RAGFlowMessageRole represents the role of a message sender.
type RAGFlowMessageRole string

const (
	RAGFlowMessageRoleUser      RAGFlowMessageRole = "user"
	RAGFlowMessageRoleAssistant RAGFlowMessageRole = "assistant"
)

// RAGFlowMessageReference represents a reference to source content in a message.
type RAGFlowMessageReference struct {
	MemoUID         string  `json:"memo_uid,omitempty"`
	AttachmentUID   string  `json:"attachment_uid,omitempty"`
	ContentSnippet  string  `json:"content_snippet"`
	SimilarityScore float64 `json:"similarity_score"`
}

// RAGFlowMessage represents a message in a RAGFlow conversation.
type RAGFlowMessage struct {
	ID             int32
	UID            string
	ConversationID int32
	Role           RAGFlowMessageRole
	Content        string
	ReferencesJSON string // JSON array of RAGFlowMessageReference
	CreatedTs      int64
}

// FindRAGFlowMessage specifies filter criteria for finding messages.
type FindRAGFlowMessage struct {
	ID             *int32
	UID            *string
	ConversationID *int32
	Limit          *int
	Offset         *int
	OrderByCreated *string // "ASC" or "DESC"
}

// DeleteRAGFlowMessage specifies which message to delete.
type DeleteRAGFlowMessage struct {
	ID             *int32
	ConversationID *int32
}

// CreateRAGFlowMessage creates a new RAGFlow message.
func (s *Store) CreateRAGFlowMessage(ctx context.Context, create *RAGFlowMessage) (*RAGFlowMessage, error) {
	return s.driver.CreateRAGFlowMessage(ctx, create)
}

// GetRAGFlowMessage returns a single message matching the filter.
func (s *Store) GetRAGFlowMessage(ctx context.Context, find *FindRAGFlowMessage) (*RAGFlowMessage, error) {
	list, err := s.driver.ListRAGFlowMessages(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

// ListRAGFlowMessages returns messages matching the filter.
func (s *Store) ListRAGFlowMessages(ctx context.Context, find *FindRAGFlowMessage) ([]*RAGFlowMessage, error) {
	return s.driver.ListRAGFlowMessages(ctx, find)
}

// DeleteRAGFlowMessage deletes messages matching the filter.
func (s *Store) DeleteRAGFlowMessage(ctx context.Context, delete *DeleteRAGFlowMessage) error {
	return s.driver.DeleteRAGFlowMessage(ctx, delete)
}
