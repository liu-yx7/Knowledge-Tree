package store

import "context"

// AIMessageRole represents the role of a message sender.
type AIMessageRole string

const (
	AIMessageRoleUser      AIMessageRole = "user"
	AIMessageRoleAssistant AIMessageRole = "assistant"
)

// AIMessage represents a message in an AI conversation.
// P3 架构：消息由 Knowtree 本地存储，支持引用信息和思考链
type AIMessage struct {
	ID             int32
	UID            string
	ConversationID int32
	Role           AIMessageRole
	Content        string

	// 思考链（DeepSeek 风格，可选）
	ReasoningContent string

	// 引用信息（JSON 数组）
	// 格式: [{"memo_uid":"...","type":"memo","content_snippet":"...","similarity":0.85}]
	ReferencesJSON string

	// Token 使用统计（JSON 对象）
	// 格式: {"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}
	TokenUsageJSON string

	CreatedTs int64
}

// FindAIMessage specifies filter criteria for finding messages.
type FindAIMessage struct {
	ID             *int32
	UID            *string
	ConversationID *int32
	Limit          *int
	Offset         *int
	OrderByCreated *string // "ASC" or "DESC"
}

// DeleteAIMessage specifies which message to delete.
type DeleteAIMessage struct {
	ID             *int32
	ConversationID *int32
}

// CreateAIMessage creates a new AI message.
func (s *Store) CreateAIMessage(ctx context.Context, create *AIMessage) (*AIMessage, error) {
	return s.driver.CreateAIMessage(ctx, create)
}

// ListAIMessages returns messages matching the filter.
func (s *Store) ListAIMessages(ctx context.Context, find *FindAIMessage) ([]*AIMessage, error) {
	return s.driver.ListAIMessages(ctx, find)
}

// DeleteAIMessage deletes messages matching the filter.
func (s *Store) DeleteAIMessage(ctx context.Context, delete *DeleteAIMessage) error {
	return s.driver.DeleteAIMessage(ctx, delete)
}
