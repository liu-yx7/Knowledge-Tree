package ragflow

import (
	"fmt"
	"os"
	"time"
)

// ==================== 默认 LLM 模型配置 ====================

// DefaultLLMModels 默认 LLM 模型列表（按优先级排序）
// 创建 Chat Assistant 时会按顺序尝试，直到找到可用模型
// 格式：{model_name}@{provider}
var DefaultLLMModels = []string{
	"qwen-plus@Tongyi-Qianwen",  // 通义千问 Plus（推荐，性价比高）
	"qwen-turbo@Tongyi-Qianwen", // 通义千问 Turbo（备选，更快）
	"qwen-max@Tongyi-Qianwen",   // 通义千问 Max（备选，更强）
	"deepseek-chat@DeepSeek",    // DeepSeek（如果用户配置了）
}

// DefaultLLMProvider 默认 LLM 提供商（与 EnsureLLMConfig 配置的一致）
const DefaultLLMProvider = "Tongyi-Qianwen"

// 默认模型 ID（Tongyi-Qianwen 系列，格式：model_name@provider）
// 用于 SetTenantInfo 设置用户的默认模型
// 格式：{model_name}@{provider}（SetTenantInfo API 需要完整格式）
const (
	DefaultEmbeddingModel  = "text-embedding-v4@Tongyi-Qianwen"
	DefaultASRModel        = "qwen3-asr-flash@Tongyi-Qianwen"
	DefaultImage2TextModel = "qwen-vl-plus@Tongyi-Qianwen"
	DefaultRerankModel     = "gte-rerank@Tongyi-Qianwen"
	DefaultTTSModel        = "sambert-zhide-v1@Tongyi-Qianwen"
)

// DefaultRerankModelName 仅模型名（不含 @provider），用于 Chat Assistant 的 rerank_id 字段。
// RAGFlow chat.py 的 rerank_id 校验不调用 split_model_name_and_factory()，
// 直接用完整的 name@provider 查询 TenantLLM.llm_name 字段会匹配失败。
const DefaultRerankModelName = "gte-rerank"

// GetDefaultLLMID 获取默认 LLM 模型 ID
// 优先级：环境变量 > 硬编码默认值
func GetDefaultLLMID() string {
	// 1. 优先使用环境变量（保持向后兼容）
	if envLLM := os.Getenv("RAGFLOW_DEFAULT_LLM_ID"); envLLM != "" {
		return envLLM
	}

	// 2. 返回硬编码默认值（qwen-plus 是最稳定的选择）
	return DefaultLLMModels[0]
}

// ==================== 配置定义 ====================

// Config RAGFlow 连接配置
type Config struct {
	// BaseURL RAGFlow 服务地址，例如 http://localhost:9380
	BaseURL string

	// APIKey 认证密钥
	APIKey string

	// AssistantID 默认助手 ID（用于聊天功能）
	AssistantID string

	// DefaultLLMID 创建 Chat Assistant 时使用的默认 LLM 模型
	// 格式：{model_name}@{provider}，例如 "deepseek-chat@DeepSeek"
	// 必须是 RAGFlow 中已配置的模型，否则创建 Assistant 会失败
	DefaultLLMID string

	// DashScopeAPIKey 百炼 API Key（用于调用百炼 API 和配置 RAGFlow LLM）
	// 从环境变量 DASHSCOPE_API_KEY 获取
	// 用途：
	// 1. 调用百炼 API 获取可用模型列表
	// 2. 配置 RAGFlow 的 Tongyi-Qianwen 模型提供商
	DashScopeAPIKey string

	// Timeout 请求超时时间
	Timeout time.Duration

	// MaxRetries 最大重试次数
	MaxRetries int

	// PublicKeyPath RSA 公钥文件路径（用于密码加密）
	// 加载优先级：PublicKeyPath > 环境变量 RAGFLOW_PUBLIC_KEY_PATH > 默认路径
	PublicKeyPath string
}

// Validate 验证配置有效性（系统级：只需要 BaseURL）
// APIKey 不是启动时的必要条件，而是运行时 per-user 动态获取
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("RAGFlow BaseURL 不能为空")
	}
	return nil
}

// ValidateForProvisioning 验证用于用户自动配置的配置有效性
// 此验证不要求 APIKey，因为 Provisioner 会自动生成用户级 API Key
func (c *Config) ValidateForProvisioning() error {
	if c.BaseURL == "" {
		return fmt.Errorf("RAGFlow BaseURL 不能为空")
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

// ==================== 公钥加载 ====================

// 默认公钥文件路径
const defaultPublicKeyPath = "./conf/ragflow_public.pem"

// LoadPublicKey 加载 RSA 公钥
// 加载优先级：
// 1. 环境变量 RAGFLOW_PUBLIC_KEY（直接包含 PEM 内容）
// 2. Config.PublicKeyPath（文件路径）
// 3. 环境变量 RAGFLOW_PUBLIC_KEY_PATH（文件路径）
// 4. 默认路径 ./conf/ragflow_public.pem
func (c *Config) LoadPublicKey() ([]byte, error) {
	// 优先级 1: 环境变量直接包含公钥内容
	if envKey := os.Getenv("RAGFLOW_PUBLIC_KEY"); envKey != "" {
		return []byte(envKey), nil
	}

	// 优先级 2: Config.PublicKeyPath
	if c.PublicKeyPath != "" {
		return loadPublicKeyFromFile(c.PublicKeyPath)
	}

	// 优先级 3: 环境变量指定文件路径
	if envPath := os.Getenv("RAGFLOW_PUBLIC_KEY_PATH"); envPath != "" {
		return loadPublicKeyFromFile(envPath)
	}

	// 优先级 4: 默认路径
	return loadPublicKeyFromFile(defaultPublicKeyPath)
}

// loadPublicKeyFromFile 从文件加载公钥
func loadPublicKeyFromFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("公钥文件不存在: %s", path)
		}
		return nil, fmt.Errorf("读取公钥文件失败 (%s): %w", path, err)
	}
	return data, nil
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
