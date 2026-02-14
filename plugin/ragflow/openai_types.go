// Package ragflow - OpenAI 兼容 API 类型定义
// 对齐 RAGFlow 源码 api/apps/sdk/session.py 中 chat_completion_openai_like 的实际格式
// 用于对话交互，管理操作仍使用 SDK API（ragflow.go / types.go）
package ragflow

// ==================== OpenAI 兼容请求 ====================

// OpenAIChatCompletionRequest OpenAI 格式的聊天补全请求
// 发送到: POST /api/v1/chats_openai/{chat_id}/chat/completions
type OpenAIChatCompletionRequest struct {
	Model     string           `json:"model"`
	Messages  []OpenAIMessage  `json:"messages"`
	Stream    bool             `json:"stream"`
	ExtraBody *OpenAIExtraBody `json:"extra_body,omitempty"`
}

// OpenAIMessage OpenAI 格式的消息
// RAGFlow 会过滤掉 role=system 和开头的 role=assistant
type OpenAIMessage struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// OpenAIExtraBody RAGFlow 扩展字段（通过 extra_body 传递）
type OpenAIExtraBody struct {
	// Reference 是否返回引用信息（对应 quote 功能）
	Reference bool `json:"reference,omitempty"`
	// Reasoning 是否启用深度研究/思考链（DeepSeek R1 等模型支持）
	// 启用后会在流式响应中返回 reasoning_content 字段
	Reasoning bool `json:"reasoning,omitempty"`
	// MetadataCondition 元数据过滤条件（用于 visibility 过滤等）
	MetadataCondition *MetadataCondition `json:"metadata_condition,omitempty"`
}

// MetadataCondition 元数据过滤条件
type MetadataCondition struct {
	Logic      string               `json:"logic"` // "and" | "or"
	Conditions []MetadataFilterItem `json:"conditions"`
}

// MetadataFilterItem 单个元数据过滤项
type MetadataFilterItem struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // "eq" | "ne" | "in" | "contains" 等
	Value    any    `json:"value"`
}

// ==================== OpenAI 兼容响应（非流式） ====================

// OpenAIChatResponse 非流式聊天补全响应
type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIChoice 响应选项
type OpenAIChoice struct {
	// Message 非流式响应中的完整消息
	Message *OpenAIResponseMessage `json:"message,omitempty"`
	// Delta 流式响应中的增量消息
	Delta        *OpenAIDelta `json:"delta,omitempty"`
	FinishReason *string      `json:"finish_reason"`
	Index        int          `json:"index"`
}

// OpenAIResponseMessage 非流式响应消息体
type OpenAIResponseMessage struct {
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	Reference []OpenAIReference `json:"reference,omitempty"`
}

// ==================== OpenAI 兼容响应（流式） ====================

// OpenAIDelta 流式响应增量体
// 对齐 RAGFlow 源码：普通 chunk 含 content/reasoning_content，
// 最后一个 chunk（finish_reason=stop）含 reference/final_content
type OpenAIDelta struct {
	Content          *string           `json:"content"`
	Role             string            `json:"role,omitempty"`
	ReasoningContent *string           `json:"reasoning_content"`
	Reference        []OpenAIReference `json:"reference,omitempty"`
	FinalContent     string            `json:"final_content,omitempty"`
}

// ==================== 引用信息 ====================

// OpenAIReference RAGFlow chunks_format 标准化后的引用
// 由 RAGFlow 内部 chunks_format() 函数生成，字段稳定
type OpenAIReference struct {
	ID               string  `json:"id"`
	Content          string  `json:"content"`
	DocumentID       string  `json:"document_id"`
	DocumentName     string  `json:"document_name"`
	DatasetID        string  `json:"dataset_id"`
	ImageID          string  `json:"image_id,omitempty"`
	Positions        [][]int `json:"positions,omitempty"`
	URL              string  `json:"url,omitempty"`
	Similarity       float64 `json:"similarity"`
	VectorSimilarity float64 `json:"vector_similarity"`
	TermSimilarity   float64 `json:"term_similarity"`
	DocType          string  `json:"doc_type,omitempty"`
}

// ==================== 流式 chunk（面向消费者） ====================

// OpenAIChatChunk 流式响应中的单个事件（已解析，面向 Service 层消费）
// 普通 chunk：Content 非空
// 最后一个 chunk：Done=true，可能含 References/FinalContent/Usage
// 错误 chunk：Error 非空
type OpenAIChatChunk struct {
	// Content 增量文本
	Content string
	// ReasoningContent 思考链增量（DeepSeek 风格，可选）
	ReasoningContent string
	// Done 是否完成
	Done bool
	// FinishReason 完成原因（"stop" 表示正常结束）
	FinishReason string
	// References 引用列表（仅最后一个 chunk 非空）
	References []OpenAIReference
	// FinalContent 完整回答（仅最后一个 chunk 非空，用于兜底）
	FinalContent string
	// Usage Token 使用统计（仅最后一个 chunk 非空）
	Usage *OpenAIUsage
	// Error 错误信息
	Error error
}

// ==================== Token 使用统计 ====================

// OpenAIUsage Token 使用统计
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ==================== 构造辅助函数 ====================

// NewOpenAIChatRequest 创建标准的 OpenAI 聊天请求（开启引用）
func NewOpenAIChatRequest(messages []OpenAIMessage, stream bool) *OpenAIChatCompletionRequest {
	return &OpenAIChatCompletionRequest{
		Model:    "model",
		Messages: messages,
		Stream:   stream,
		ExtraBody: &OpenAIExtraBody{
			Reference: true,
		},
	}
}

// ==================== 辅助函数 ====================

// derefString 安全解引用字符串指针，nil 返回空字符串
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// stringPtr 返回字符串的指针
func stringPtr(s string) *string {
	return &s
}
