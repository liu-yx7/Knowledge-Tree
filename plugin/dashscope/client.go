// Package dashscope 提供阿里云百炼 (DashScope) API 客户端
// 主要用于获取可用的 LLM 模型列表，供用户在前端选择
// 使用 OpenAI 兼容模式 API: /compatible-mode/v1/models
package dashscope

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ==================== 配置定义 ====================

// Config DashScope 客户端配置
type Config struct {
	// APIKey 百炼 API Key（从环境变量 DASHSCOPE_API_KEY 获取）
	APIKey string

	// BaseURL API 基础地址（默认 https://dashscope.aliyuncs.com）
	BaseURL string

	// Timeout 请求超时时间（默认 30 秒）
	Timeout time.Duration

	// CacheTTL 模型列表缓存时间（默认 5 分钟）
	CacheTTL time.Duration
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:  "https://dashscope.aliyuncs.com",
		Timeout:  30 * time.Second,
		CacheTTL: 5 * time.Minute,
	}
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("DashScope API Key 不能为空")
	}
	return nil
}

// WithDefaults 填充默认值
func (c *Config) WithDefaults() *Config {
	if c.BaseURL == "" {
		c.BaseURL = "https://dashscope.aliyuncs.com"
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.CacheTTL == 0 {
		c.CacheTTL = 5 * time.Minute
	}
	return c
}

// ==================== API 响应类型 (OpenAI 兼容模式) ====================

// OpenAIModelsResponse OpenAI 兼容模式的模型列表响应
// GET /compatible-mode/v1/models
type OpenAIModelsResponse struct {
	Object  string        `json:"object"`   // "list"
	Data    []OpenAIModel `json:"data"`     // 模型列表
	FirstID string        `json:"first_id"` // 首个模型 ID
	LastID  string        `json:"last_id"`  // 最后模型 ID
	HasMore bool          `json:"has_more"` // 是否有更多
}

// OpenAIModel OpenAI 兼容模式的模型信息
type OpenAIModel struct {
	ID      string `json:"id"`       // 模型 ID (如 "qwen-plus", "deepseek-r1")
	Object  string `json:"object"`   // "model"
	Created int64  `json:"created"`  // 创建时间戳
	OwnedBy string `json:"owned_by"` // 所有者 (如 "system")
}

// Model 内部统一的模型信息结构
type Model struct {
	// ModelID 模型唯一标识（与 ModelName 相同）
	ModelID string

	// ModelName 模型名称（用于 API 调用，如 "qwen-plus"）
	ModelName string

	// DisplayName 显示名称（用于前端展示）
	DisplayName string

	// ModelType 模型类型：text-generation, embeddings, multimodal 等
	ModelType string

	// Status 状态：RUNNING, STOPPED 等
	Status string

	// OwnedBy 所有者
	OwnedBy string

	// Description 描述
	Description string

	// CreateTime 创建时间
	CreateTime string
}

// ==================== 客户端定义 ====================

// Client DashScope API 客户端
type Client struct {
	config     *Config
	httpClient *http.Client

	// 模型列表缓存
	cache      []Model
	cacheTime  time.Time
	cacheMutex sync.RWMutex
}

// NewClient 创建 DashScope 客户端
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg = cfg.WithDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// ==================== 模型列表 ====================

// ragflowRegisteredChatModels 是 RAGFlow llm_factories.json 中
// Tongyi-Qianwen 工厂下已注册的 chat 类模型精确名称集合。
//
// 设计约束：
//   - ListAvailableModels 只能暴露此集合内的模型，确保用户选择的任何模型
//     在 RAGFlow TenantLLM 表中都有对应记录，避免创建 Assistant 时报
//     "`model_name` xxx doesn't exist"。
//   - DashScope API 可能返回更多模型（新上线但 RAGFlow 尚未注册），
//     这些模型通过此白名单被屏蔽，直到 llm_factories.json 补充后同步更新此处。
//
// 维护说明：
//
//	当 ragflow/conf/llm_factories.json 的 Tongyi-Qianwen.llm 新增 chat 模型时，
//	同步在此集合追加对应模型名。
var ragflowRegisteredChatModels = map[string]struct{}{
	// ── Moonshot（通过百炼托管）──
	"Moonshot-Kimi-K2-Instruct": {},
	// ── DeepSeek 系列 ──
	"deepseek-v3.2":                 {},
	"deepseek-r1":                   {},
	"deepseek-v3":                   {},
	"deepseek-r1-distill-qwen-1.5b": {},
	"deepseek-r1-distill-qwen-7b":   {},
	"deepseek-r1-distill-qwen-14b":  {},
	"deepseek-r1-distill-qwen-32b":  {},
	"deepseek-r1-distill-llama-8b":  {},
	"deepseek-r1-distill-llama-70b": {},
	// ── QwQ 系列 ──
	"qwq-32b":         {},
	"qwq-plus":        {},
	"qwq-plus-latest": {},
	// ── Qwen Flash 系列 ──
	"qwen-flash":            {},
	"qwen-flash-2025-07-28": {},
	// ── Qwen Plus 系列 ──
	"qwen-plus":            {},
	"qwen-plus-latest":     {},
	"qwen-plus-2025-04-28": {},
	"qwen-plus-2025-07-14": {},
	"qwen-plus-2025-07-28": {},
	// ── Qwen Max 系列 ──
	"qwen-max": {},
	// ── Qwen Turbo 系列 ──
	"qwen-turbo":            {},
	"qwen-turbo-latest":     {},
	"qwen-turbo-2025-04-28": {},
	// ── Qwen Long ──
	"qwen-long": {},
	// ── Qwen3 Max ──
	"qwen3-max": {},
	// ── Qwen3 235B ──
	"qwen3-235b-a22b":               {},
	"qwen3-235b-a22b-instruct-2507": {},
	"qwen3-235b-a22b-thinking-2507": {},
	// ── Qwen3 30B ──
	"qwen3-30b-a3b":               {},
	"qwen3-30b-a3b-instruct-2507": {},
	"qwen3-30b-a3b-thinking-2507": {},
	// ── Qwen3 Next 80B ──
	"qwen3-next-80b-a3b-instruct": {},
	"qwen3-next-80b-a3b-thinking": {},
	// ── Qwen3 小尺寸系列 ──
	"qwen3-0.6b": {},
	"qwen3-1.7b": {},
	"qwen3-4b":   {},
	"qwen3-8b":   {},
	"qwen3-14b":  {},
	"qwen3-32b":  {},
	// ── 深度研究 ──
	"qianwen-deepresearch-30b-a3b-131k": {},
}

// isRAGFlowRegistered 检查模型是否已在 RAGFlow Tongyi-Qianwen 工厂中注册
// 只有注册过的模型才能被 RAGFlow TenantLLM 表接受，进而用于创建 Assistant
func isRAGFlowRegistered(modelName string) bool {
	_, ok := ragflowRegisteredChatModels[modelName]
	return ok
}

// IsRAGFlowRegistered 是 isRAGFlowRegistered 的导出版本，供外部包使用
// （如 server/router/api/v1 的 SetUserLLMPreference 服务端校验）
func IsRAGFlowRegistered(modelName string) bool {
	return isRAGFlowRegistered(modelName)
}

// ListModels 获取可用模型列表（带缓存）
// 优先返回缓存数据，缓存过期后自动刷新
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	// 快速路径：检查缓存
	c.cacheMutex.RLock()
	if c.cache != nil && time.Since(c.cacheTime) < c.config.CacheTTL {
		models := c.cache
		c.cacheMutex.RUnlock()
		return models, nil
	}
	c.cacheMutex.RUnlock()

	// 慢路径：从 API 获取
	models, err := c.fetchModels(ctx)
	if err != nil {
		// 如果 API 失败但有旧缓存，返回旧缓存
		c.cacheMutex.RLock()
		if c.cache != nil {
			oldCache := c.cache
			c.cacheMutex.RUnlock()
			slog.Warn("DashScope API 调用失败，使用缓存数据", slog.Any("error", err))
			return oldCache, nil
		}
		c.cacheMutex.RUnlock()
		return nil, err
	}

	// 更新缓存
	c.cacheMutex.Lock()
	c.cache = models
	c.cacheTime = time.Now()
	c.cacheMutex.Unlock()

	return models, nil
}

// ListChatModels 获取可用的聊天模型列表
// 双重过滤规则：
//  1. isChatModel：排除 embedding/TTS/ASR/视觉等非对话模型
//  2. isRAGFlowRegistered：仅保留已在 llm_factories.json 注册的模型，
//     确保前端展示的每个模型都能被 RAGFlow 接受（TenantLLM 表有记录）
func (c *Client) ListChatModels(ctx context.Context) ([]Model, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	var chatModels []Model
	for _, m := range models {
		// 第一道过滤：模型类型（排除非对话类）
		if !isChatModel(m.ModelName) {
			continue
		}
		// 第二道过滤：RAGFlow 注册表白名单
		// 只有 llm_factories.json 中注册了的模型，RAGFlow TenantLLM 表才会有记录，
		// Assistant 才能用该模型创建成功。
		if !isRAGFlowRegistered(m.ModelName) {
			slog.Debug("ListChatModels: 跳过未在 RAGFlow 注册的模型",
				slog.String("model", m.ModelName))
			continue
		}
		chatModels = append(chatModels, m)
	}

	slog.Info("ListChatModels: 过滤后的聊天模型",
		slog.Int("totalModels", len(models)),
		slog.Int("chatModels", len(chatModels)))

	return chatModels, nil
}

// isChatModel 判断是否为聊天类模型
// 基于模型名称前缀进行判断
func isChatModel(modelName string) bool {
	name := strings.ToLower(modelName)

	// 聊天模型前缀白名单
	chatPrefixes := []string{
		"qwen-plus", "qwen-max", "qwen-turbo", "qwen-long", "qwen-flash",
		"qwen2", "qwen3", "qwq-", "qvq-",
		"deepseek-r1", "deepseek-v3", "deepseek-chat",
		"glm-", "kimi-",
		"minimax",
	}

	for _, prefix := range chatPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	// 排除非聊天模型（TTS、ASR、VL、图像、翻译等）
	excludePatterns := []string{
		"-tts", "-asr", "-vl-", "-ocr", "-image", "-mt-",
		"-omni-", "-coder", "-math", "-embed",
	}

	for _, pattern := range excludePatterns {
		if strings.Contains(name, pattern) {
			return false
		}
	}

	// 兜底：包含 instruct 的通常是聊天模型
	if strings.Contains(name, "instruct") {
		return true
	}

	return false
}

// RefreshCache 强制刷新模型缓存
func (c *Client) RefreshCache(ctx context.Context) error {
	models, err := c.fetchModels(ctx)
	if err != nil {
		return err
	}

	c.cacheMutex.Lock()
	c.cache = models
	c.cacheTime = time.Now()
	c.cacheMutex.Unlock()

	return nil
}

// InvalidateCache 清除缓存
func (c *Client) InvalidateCache() {
	c.cacheMutex.Lock()
	c.cache = nil
	c.cacheTime = time.Time{}
	c.cacheMutex.Unlock()
}

// ==================== 内部方法 ====================

// fetchModels 从 OpenAI 兼容模式 API 获取模型列表
func (c *Client) fetchModels(ctx context.Context) ([]Model, error) {
	// 使用 OpenAI 兼容模式端点
	url := fmt.Sprintf("%s/compatible-mode/v1/models", c.config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result OpenAIModelsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为内部 Model 结构
	models := make([]Model, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, Model{
			ModelID:     m.ID,
			ModelName:   m.ID, // OpenAI 兼容模式下 ID 就是模型名
			DisplayName: formatDisplayName(m.ID),
			ModelType:   inferModelType(m.ID),
			Status:      "RUNNING", // OpenAI 兼容模式返回的都是可用模型
			OwnedBy:     m.OwnedBy,
			CreateTime:  time.Unix(m.Created, 0).Format(time.RFC3339),
		})
	}

	slog.Info("DashScope API 获取模型列表成功",
		slog.Int("count", len(models)))

	return models, nil
}

// formatDisplayName 格式化模型显示名称
func formatDisplayName(modelID string) string {
	// 将模型 ID 转换为更友好的显示名称
	name := modelID

	// 处理常见前缀
	replacements := map[string]string{
		"qwen-plus":   "通义千问 Plus",
		"qwen-max":    "通义千问 Max",
		"qwen-turbo":  "通义千问 Turbo",
		"qwen-long":   "通义千问 Long",
		"qwen-flash":  "通义千问 Flash",
		"qwq-plus":    "通义千问 QwQ Plus",
		"qvq-plus":    "通义千问 QvQ Plus",
		"qvq-max":     "通义千问 QvQ Max",
		"deepseek-r1": "DeepSeek R1",
		"deepseek-v3": "DeepSeek V3",
		"glm-4":       "智谱 GLM-4",
		"kimi-k2":     "Kimi K2",
	}

	for prefix, displayPrefix := range replacements {
		if strings.HasPrefix(strings.ToLower(modelID), prefix) {
			suffix := modelID[len(prefix):]
			if suffix == "" {
				return displayPrefix
			}
			return displayPrefix + " " + strings.TrimPrefix(suffix, "-")
		}
	}

	return name
}

// inferModelType 推断模型类型
func inferModelType(modelID string) string {
	name := strings.ToLower(modelID)

	if strings.Contains(name, "-tts") || strings.Contains(name, "-asr") {
		return "audio"
	}
	if strings.Contains(name, "-vl-") || strings.Contains(name, "-ocr") {
		return "vision"
	}
	if strings.Contains(name, "-image") || strings.Contains(name, "wanx") {
		return "image-generation"
	}
	if strings.Contains(name, "-embed") {
		return "embeddings"
	}
	if strings.Contains(name, "-mt-") {
		return "translation"
	}

	return "text-generation"
}

// GetModel 根据模型名称获取模型信息
func (c *Client) GetModel(ctx context.Context, modelName string) (*Model, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	for _, m := range models {
		if m.ModelName == modelName || m.ModelID == modelName {
			return &m, nil
		}
	}

	return nil, fmt.Errorf("模型不存在: %s", modelName)
}

// ModelExists 检查模型是否存在且可用
func (c *Client) ModelExists(ctx context.Context, modelName string) bool {
	model, err := c.GetModel(ctx, modelName)
	if err != nil {
		return false
	}
	return model.Status == "RUNNING"
}
