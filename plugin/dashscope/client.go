// Package dashscope 提供阿里云百炼 (DashScope) API 客户端
// 主要用于获取可用的 LLM 模型列表，供用户在前端选择
package dashscope

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// ==================== API 响应类型 ====================

// ModelsResponse 模型列表 API 响应
// GET /api/v1/deployments/models
type ModelsResponse struct {
	Output struct {
		Data       []Model `json:"data"`
		Total      int     `json:"total"`
		PageNo     int     `json:"page_no"`
		PageSize   int     `json:"page_size"`
		TotalPages int     `json:"total_pages"`
	} `json:"output"`
	RequestID string `json:"request_id"`
}

// Model 模型信息
type Model struct {
	// ModelID 模型唯一标识
	ModelID string `json:"model_id"`

	// ModelName 模型名称（用于 API 调用）
	ModelName string `json:"model_name"`

	// DisplayName 显示名称（用于前端展示）
	DisplayName string `json:"display_name"`

	// ModelType 模型类型：text-generation, embeddings, multimodal 等
	ModelType string `json:"model_type"`

	// Status 状态：RUNNING, STOPPED 等
	Status string `json:"status"`

	// OwnedBy 所有者
	OwnedBy string `json:"owned_by"`

	// Description 描述
	Description string `json:"description"`

	// CreateTime 创建时间
	CreateTime string `json:"create_time"`
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

// ListChatModels 获取可用的聊天模型列表（过滤 text-generation 类型）
func (c *Client) ListChatModels(ctx context.Context) ([]Model, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	var chatModels []Model
	for _, m := range models {
		// 只返回状态为 RUNNING 的文本生成模型
		if m.ModelType == "text-generation" && m.Status == "RUNNING" {
			chatModels = append(chatModels, m)
		}
	}

	return chatModels, nil
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

// fetchModels 从 API 获取模型列表
func (c *Client) fetchModels(ctx context.Context) ([]Model, error) {
	// 分页获取所有模型
	var allModels []Model
	pageNo := 1
	pageSize := 100

	for {
		url := fmt.Sprintf("%s/api/v1/deployments/models?page_no=%d&page_size=%d",
			c.config.BaseURL, pageNo, pageSize)

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

		var result ModelsResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		allModels = append(allModels, result.Output.Data...)

		// 检查是否还有更多页
		if pageNo >= result.Output.TotalPages || len(result.Output.Data) == 0 {
			break
		}
		pageNo++
	}

	return allModels, nil
}

// GetModel 根据模型名称获取模型信息
func (c *Client) GetModel(ctx context.Context, modelName string) (*Model, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	for _, m := range models {
		if m.ModelName == modelName {
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
