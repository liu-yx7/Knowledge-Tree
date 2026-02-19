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
// 过滤规则：只返回文本对话类模型（qwen-*, deepseek-*, glm-*, kimi-* 等）
func (c *Client) ListChatModels(ctx context.Context) ([]Model, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	var chatModels []Model
	for _, m := range models {
		// 只返回聊天类模型（基于模型名称前缀判断）
		if isChatModel(m.ModelName) {
			chatModels = append(chatModels, m)
		}
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
