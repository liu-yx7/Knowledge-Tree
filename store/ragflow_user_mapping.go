package store

import "context"

// RAGFlowUserMapping represents the mapping between a user and their RAGFlow dataset/assistant.
type RAGFlowUserMapping struct {
	ID              int32
	UserID          int32
	DatasetID       string
	DatasetName     string
	AssistantID     string
	DocumentCount   int32
	LastSyncTs      *int64
	RAGFlowUserID   string
	RAGFlowEmail    string
	RAGFlowPassword string
	APIKey          string
	// LLMConfigured 标记用户是否已配置 LLM 提供商（百炼）
	// P4 新增字段，用于跳过重复配置
	LLMConfigured bool
	// PreferredLLMID 用户偏好的 LLM 模型 ID
	// 格式：{model_name}@{provider}，例如 "qwen-max@Tongyi-Qianwen"
	PreferredLLMID string
	// DatasetIDs 用户选择的 Dataset ID 列表（JSON 数组字符串）
	// 例如：["kb_001", "kb_002"]
	DatasetIDs string
	// QuoteEnabled RAG 引用开关（默认 true）
	// P4 新增字段，控制对话时是否返回引用信息
	QuoteEnabled bool
	// ReasoningEnabled 深度研究（Reasoning）开关（默认 false）
	// P4 新增字段，控制对话时是否启用深度研究功能
	ReasoningEnabled bool
	CreatedTs        int64
	UpdatedTs        int64
}

// FindRAGFlowUserMapping specifies filter criteria for finding user mappings.
type FindRAGFlowUserMapping struct {
	ID        *int32
	UserID    *int32
	DatasetID *string
}

// UpdateRAGFlowUserMapping specifies fields to update.
type UpdateRAGFlowUserMapping struct {
	ID              int32
	DatasetID       *string
	DatasetName     *string
	AssistantID     *string
	DocumentCount   *int32
	LastSyncTs      *int64
	RAGFlowUserID   *string
	RAGFlowEmail    *string
	RAGFlowPassword *string
	APIKey          *string
	// LLMConfigured 标记用户是否已配置 LLM 提供商
	LLMConfigured *bool
	// PreferredLLMID 用户偏好的 LLM 模型 ID
	PreferredLLMID *string
	// DatasetIDs 用户选择的 Dataset ID 列表
	DatasetIDs *string
	// QuoteEnabled RAG 引用开关
	QuoteEnabled *bool
	// ReasoningEnabled 深度研究开关
	ReasoningEnabled *bool
	UpdatedTs        *int64
}

// DeleteRAGFlowUserMapping specifies which mapping to delete.
type DeleteRAGFlowUserMapping struct {
	ID     *int32
	UserID *int32
}

// CreateRAGFlowUserMapping creates a new RAGFlow user mapping.
func (s *Store) CreateRAGFlowUserMapping(ctx context.Context, create *RAGFlowUserMapping) (*RAGFlowUserMapping, error) {
	return s.driver.CreateRAGFlowUserMapping(ctx, create)
}

// GetRAGFlowUserMapping returns a single user mapping matching the filter.
func (s *Store) GetRAGFlowUserMapping(ctx context.Context, find *FindRAGFlowUserMapping) (*RAGFlowUserMapping, error) {
	list, err := s.driver.ListRAGFlowUserMappings(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

// ListRAGFlowUserMappings returns user mappings matching the filter.
func (s *Store) ListRAGFlowUserMappings(ctx context.Context, find *FindRAGFlowUserMapping) ([]*RAGFlowUserMapping, error) {
	return s.driver.ListRAGFlowUserMappings(ctx, find)
}

// UpdateRAGFlowUserMapping updates a user mapping.
func (s *Store) UpdateRAGFlowUserMapping(ctx context.Context, update *UpdateRAGFlowUserMapping) error {
	return s.driver.UpdateRAGFlowUserMapping(ctx, update)
}

// DeleteRAGFlowUserMapping deletes a user mapping.
func (s *Store) DeleteRAGFlowUserMapping(ctx context.Context, delete *DeleteRAGFlowUserMapping) error {
	return s.driver.DeleteRAGFlowUserMapping(ctx, delete)
}
