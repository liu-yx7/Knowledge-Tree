package ragflow

import (
	"fmt"
	"time"
)

// ==================== 配置定义 ====================

// Config RAGFlow 连接配置
type Config struct {
	// BaseURL RAGFlow 服务地址，例如 http://localhost:9380
	BaseURL string

	// APIKey 认证密钥
	APIKey string

	// AssistantID 默认助手 ID（用于聊天功能）
	AssistantID string

	// Timeout 请求超时时间
	Timeout time.Duration

	// MaxRetries 最大重试次数
	MaxRetries int
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("RAGFlow BaseURL 不能为空")
	}
	if c.APIKey == "" {
		return fmt.Errorf("RAGFlow APIKey 不能为空")
	}
	return nil
}

// WithDefaults 填充默认值
func (c *Config) WithDefaults() *Config {
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	return c
}

// ==================== 分块方法枚举 ====================

// ChunkMethod 分块方法枚举
type ChunkMethod string

const (
	// ChunkMethodNaive 朴素分块
	ChunkMethodNaive ChunkMethod = "naive"
	// ChunkMethodManual 手动分块
	ChunkMethodManual ChunkMethod = "manual"
	// ChunkMethodQA 问答分块
	ChunkMethodQA ChunkMethod = "qa"
	// ChunkMethodTable 表格分块
	ChunkMethodTable ChunkMethod = "table"
	// ChunkMethodPaper 论文分块
	ChunkMethodPaper ChunkMethod = "paper"
	// ChunkMethodBook 书籍分块
	ChunkMethodBook ChunkMethod = "book"
	// ChunkMethodLaws 法律分块
	ChunkMethodLaws ChunkMethod = "laws"
	// ChunkMethodPresentation 演示文稿分块
	ChunkMethodPresentation ChunkMethod = "presentation"
	// ChunkMethodPicture 图片分块
	ChunkMethodPicture ChunkMethod = "picture"
	// ChunkMethodOne 单块
	ChunkMethodOne ChunkMethod = "one"
	// ChunkMethodKnowledgeGraph 知识图谱分块
	ChunkMethodKnowledgeGraph ChunkMethod = "knowledge_graph"
	// ChunkMethodEmail 邮件分块
	ChunkMethodEmail ChunkMethod = "email"
)
