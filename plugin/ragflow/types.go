package ragflow

import "time"

// ==================== API 响应包装 ====================

// APIResponse RAGFlow API 通用响应结构
type APIResponse[T any] struct {
	Code    int    `json:"code"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

// ==================== 列表查询选项 ====================

// ListOptions 列表查询通用选项
type ListOptions struct {
	Page     int
	PageSize int
	Name     string
	Keywords string
}

// DefaultListOptions 返回默认列表选项
func DefaultListOptions() *ListOptions {
	return &ListOptions{
		Page:     1,
		PageSize: 30,
	}
}

// ==================== 数据集相关类型 ====================

// CreateDatasetRequest 创建数据集请求
type CreateDatasetRequest struct {
	Name         string
	Description  string
	ChunkMethod  ChunkMethod
	ParserConfig map[string]any
}

// Dataset 数据集信息
// 注意：RAGFlow API 返回的 create_time/update_time 是毫秒级 Unix 时间戳（数字类型）
type Dataset struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ChunkMethod   string `json:"chunk_method"`
	ChunkCount    int    `json:"chunk_count"`
	DocumentCount int    `json:"document_count"`
	CreateTime    int64  `json:"create_time"`  // 毫秒级 Unix 时间戳
	UpdateTime    int64  `json:"update_time"`  // 毫秒级 Unix 时间戳
	CreateDate    string `json:"create_date"`  // 可读日期格式
	UpdateDate    string `json:"update_date"`  // 可读日期格式
}

// ==================== 文档相关类型 ====================

// Document 待上传文档
type Document struct {
	Name     string
	Content  []byte
	MimeType string
}

// NewTextDocument 创建文本文档
func NewTextDocument(name, content string) *Document {
	return &Document{
		Name:     name + ".txt",
		Content:  []byte(content),
		MimeType: "text/plain",
	}
}

// DocumentInfo 文档信息
// 注意：RAGFlow 返回的时间字段可能是数字或字符串，这里使用 interface{} 兼容
type DocumentInfo struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Size       int64          `json:"size"`
	ChunkCount int            `json:"chunk_count"`
	Status     string         `json:"run"`              // RAGFlow 使用 "run" 字段表示状态
	Metadata   map[string]any `json:"meta_fields,omitempty"`
	DatasetID  string         `json:"dataset_id"`
	Location   string         `json:"location"`
	Suffix     string         `json:"suffix"`
	Type       string         `json:"type"`
}

// UpdateDocumentRequest 更新文档请求
type UpdateDocumentRequest struct {
	Name         string         // 新文档名（可选）
	Metadata     map[string]any // 元数据字段，如 {"visibility": "PUBLIC"}
	ChunkMethod  string         // 分块方法（可选）
	ParserConfig map[string]any // 解析配置（可选）
}

// DocumentStatus 文档状态枚举
type DocumentStatus string

const (
	DocumentStatusPending   DocumentStatus = "pending"
	DocumentStatusParsing   DocumentStatus = "parsing"
	DocumentStatusCompleted DocumentStatus = "completed"
	DocumentStatusFailed    DocumentStatus = "failed"
)

// ==================== 检索相关类型 ====================

// RetrievalRequest 检索请求
type RetrievalRequest struct {
	DatasetIDs          []string
	Question            string
	TopK                int
	SimilarityThreshold float64
	KeywordWeight       float64
	DocumentIDs         []string
}

// DefaultRetrievalRequest 创建默认检索请求
func DefaultRetrievalRequest(datasetIDs []string, question string) *RetrievalRequest {
	return &RetrievalRequest{
		DatasetIDs:          datasetIDs,
		Question:            question,
		TopK:                6,
		SimilarityThreshold: 0.1,
		KeywordWeight:       0.3,
	}
}

// RetrievalResult 检索结果
type RetrievalResult struct {
	Chunks []Chunk `json:"chunks"`
	Total  int     `json:"total"`
}

// Chunk 检索到的文档块
type Chunk struct {
	ID           string   `json:"id"`
	Content      string   `json:"content"`
	DocumentID   string   `json:"document_id"`
	DocumentName string   `json:"document_name"`
	DatasetID    string   `json:"dataset_id"`
	Similarity   float64  `json:"similarity"`
	Positions    [][]int  `json:"positions"`
	Keywords     []string `json:"keywords"`
}

// ==================== 聊天相关类型 ====================

// Conversation 会话信息
type Conversation struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
}

// Message 聊天消息
type Message struct {
	ID         string    `json:"id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	CreateTime time.Time `json:"create_time"`
}

// MessageChunk 流式响应块
type MessageChunk struct {
	Content      string `json:"content"`
	Done         bool   `json:"done"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	SessionID string
	Question  string
	Stream    bool
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Answer     string  `json:"answer"`
	References []Chunk `json:"references"`
}
