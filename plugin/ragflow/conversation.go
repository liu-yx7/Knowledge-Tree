package ragflow

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ==================== 聊天助手管理 ====================

// CreateChatAssistantRequest 创建聊天助手的请求参数
type CreateChatAssistantRequest struct {
	Name       string   // 助手名称
	DatasetIDs []string // 关联的 Dataset ID 列表
	LLMID      string   // LLM 模型标识，格式：{model_name}@{provider}，例如 "deepseek-chat@DeepSeek"
	RerankID   string   // Rerank 模型标识，格式：{model_name}@{provider}，例如 "gte-rerank@Tongyi-Qianwen"
}

// CreateChatAssistant 创建聊天助手
// RAGFlow API: POST /api/v1/chats
// 注意：必须指定 llm_id（通过 llm.model_name 字段），否则 Assistant 无法进行对话
func (c *Client) CreateChatAssistant(ctx context.Context, req *CreateChatAssistantRequest) (*ChatAssistant, error) {
	payload := map[string]any{
		"name":        req.Name,
		"dataset_ids": req.DatasetIDs,
	}

	// 设置 LLM 模型配置（必须，否则对话时报 "Model(@None) not authorized"）
	if req.LLMID != "" {
		payload["llm"] = map[string]any{
			"model_name": req.LLMID,
		}
	}

	// 设置 Rerank 模型（提升检索准确性，为 dialog 表顶层字段）
	if req.RerankID != "" {
		payload["rerank_id"] = req.RerankID
	}

	resp, err := c.request(ctx, http.MethodPost, "/api/v1/chats", payload)
	if err != nil {
		return nil, err
	}

	return decodeResponse[ChatAssistant](resp)
}

// UpdateChatAssistant 更新聊天助手配置
// RAGFlow API: PUT /api/v1/chats/{chat_id}
// 主要用于将有内容的 Dataset 关联到已创建的 Assistant
func (c *Client) UpdateChatAssistant(ctx context.Context, chatID string, update map[string]any) error {
	path := fmt.Sprintf("/api/v1/chats/%s", chatID)

	resp, err := c.request(ctx, http.MethodPut, path, update)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// PUT 成功返回 code=0，无需解析 data
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("RAGFlow 错误 (code=%d): %s", result.Code, result.Message)
	}

	return nil
}

// UpdateAssistantLLMRequest 更新 Assistant LLM 配置的请求
type UpdateAssistantLLMRequest struct {
	// ModelName LLM 模型标识，格式：{model_name}@{provider}
	// 例如 "qwen-max@Tongyi-Qianwen"
	ModelName string

	// Temperature 温度参数（可选，0.0-2.0）
	Temperature *float64

	// TopP Top-P 参数（可选，0.0-1.0）
	TopP *float64

	// MaxTokens 最大生成 token 数（可选）
	MaxTokens *int
}

// UpdateAssistantLLM 更新 Assistant 的 LLM 模型配置
// 用于用户切换模型时同步更新 RAGFlow Assistant
func (c *Client) UpdateAssistantLLM(ctx context.Context, chatID string, req *UpdateAssistantLLMRequest) error {
	if chatID == "" {
		return fmt.Errorf("chatID 不能为空")
	}
	if req.ModelName == "" {
		return fmt.Errorf("ModelName 不能为空")
	}

	llmConfig := map[string]any{
		"model_name": req.ModelName,
	}
	if req.Temperature != nil {
		llmConfig["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		llmConfig["top_p"] = *req.TopP
	}
	if req.MaxTokens != nil {
		llmConfig["max_tokens"] = *req.MaxTokens
	}

	update := map[string]any{
		"llm": llmConfig,
	}

	return c.UpdateChatAssistant(ctx, chatID, update)
}

// UpdateAssistantDatasets 更新 Assistant 关联的 Dataset 列表
// 用于用户切换 Dataset 时同步更新 RAGFlow Assistant
func (c *Client) UpdateAssistantDatasets(ctx context.Context, chatID string, datasetIDs []string) error {
	if chatID == "" {
		return fmt.Errorf("chatID 不能为空")
	}

	update := map[string]any{
		"dataset_ids": datasetIDs,
	}

	return c.UpdateChatAssistant(ctx, chatID, update)
}

// UpdateAssistantConfig 更新 Assistant 的完整配置（LLM + Datasets）
// 用于一次性更新多个配置项
type UpdateAssistantConfigRequest struct {
	// LLM 配置（可选）
	LLM *UpdateAssistantLLMRequest

	// DatasetIDs Dataset ID 列表（可选）
	DatasetIDs []string
}

// UpdateAssistantConfig 更新 Assistant 的完整配置
func (c *Client) UpdateAssistantConfig(ctx context.Context, chatID string, req *UpdateAssistantConfigRequest) error {
	if chatID == "" {
		return fmt.Errorf("chatID 不能为空")
	}

	update := make(map[string]any)

	// 设置 LLM 配置
	if req.LLM != nil && req.LLM.ModelName != "" {
		llmConfig := map[string]any{
			"model_name": req.LLM.ModelName,
		}
		if req.LLM.Temperature != nil {
			llmConfig["temperature"] = *req.LLM.Temperature
		}
		if req.LLM.TopP != nil {
			llmConfig["top_p"] = *req.LLM.TopP
		}
		if req.LLM.MaxTokens != nil {
			llmConfig["max_tokens"] = *req.LLM.MaxTokens
		}
		update["llm"] = llmConfig
	}

	// 设置 Dataset IDs
	if req.DatasetIDs != nil {
		update["dataset_ids"] = req.DatasetIDs
	}

	if len(update) == 0 {
		return nil // 没有需要更新的内容
	}

	return c.UpdateChatAssistant(ctx, chatID, update)
}

// ListChatAssistants 列出聊天助手
// RAGFlow API: GET /api/v1/chats
func (c *Client) ListChatAssistants(ctx context.Context, opts *ListOptions) ([]ChatAssistant, error) {
	path := fmt.Sprintf("/api/v1/chats?page=%d&page_size=%d", opts.Page, opts.PageSize)
	if opts.Name != "" {
		path += "&name=" + opts.Name
	}

	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	result, err := decodeResponse[[]ChatAssistant](resp)
	if err != nil {
		return nil, err
	}

	return *result, nil
}

// DeleteChatAssistant 删除聊天助手
// RAGFlow API: DELETE /api/v1/chats
func (c *Client) DeleteChatAssistant(ctx context.Context, id string) error {
	payload := map[string]any{
		"ids": []string{id},
	}

	resp, err := c.request(ctx, http.MethodDelete, "/api/v1/chats", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除聊天助手失败: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// ==================== 会话管理 ====================

// CreateSession 创建会话
// RAGFlow API: POST /api/v1/chats/{chat_id}/sessions
func (c *Client) CreateSession(ctx context.Context, chatID string, name string) (*Session, error) {
	path := fmt.Sprintf("/api/v1/chats/%s/sessions", chatID)
	payload := map[string]any{}
	if name != "" {
		payload["name"] = name
	}

	resp, err := c.request(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}

	return decodeResponse[Session](resp)
}

// ListSessions 列出会话
// RAGFlow API: GET /api/v1/chats/{chat_id}/sessions
func (c *Client) ListSessions(ctx context.Context, chatID string, opts *ListOptions) ([]Session, error) {
	path := fmt.Sprintf("/api/v1/chats/%s/sessions?page=%d&page_size=%d",
		chatID, opts.Page, opts.PageSize)

	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	result, err := decodeResponse[[]Session](resp)
	if err != nil {
		return nil, err
	}

	return *result, nil
}

// DeleteSession 删除会话
// RAGFlow API: DELETE /api/v1/chats/{chat_id}/sessions
func (c *Client) DeleteSession(ctx context.Context, chatID, sessionID string) error {
	path := fmt.Sprintf("/api/v1/chats/%s/sessions", chatID)
	payload := map[string]any{
		"ids": []string{sessionID},
	}

	resp, err := c.request(ctx, http.MethodDelete, path, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除会话失败: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// ==================== 聊天交互 ====================

// Chat 发送聊天消息（非流式）
// RAGFlow API: POST /api/v1/chats/{chat_id}/completions
func (c *Client) Chat(ctx context.Context, chatID string, req *ChatRequest) (*ChatResponse, error) {
	path := fmt.Sprintf("/api/v1/chats/%s/completions", chatID)
	payload := map[string]any{
		"session_id": req.SessionID,
		"question":   req.Question,
		"stream":     false,
	}

	resp, err := c.request(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}

	return decodeResponse[ChatResponse](resp)
}

// ChatStream 发送聊天消息（流式）
// RAGFlow API: POST /api/v1/chats/{chat_id}/completions (stream=true)
// 返回一个 channel，用于接收流式响应
func (c *Client) ChatStream(ctx context.Context, chatID string, req *ChatRequest) (<-chan MessageChunk, error) {
	path := fmt.Sprintf("/api/v1/chats/%s/completions", chatID)
	url := c.config.BaseURL + path

	payload := map[string]any{
		"session_id": req.SessionID,
		"question":   req.Question,
		"stream":     true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 创建 channel 并启动 goroutine 读取 SSE 流
	chunkChan := make(chan MessageChunk, 100)

	go func() {
		defer close(chunkChan)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					chunkChan <- MessageChunk{
						Done:         true,
						FinishReason: "error",
					}
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// SSE 格式: data: {...}
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					chunkChan <- MessageChunk{
						Done:         true,
						FinishReason: "stop",
					}
					return
				}

				var chunk streamResponse
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}

				chunkChan <- MessageChunk{
					Content: chunk.Data.Answer,
					Done:    false,
				}
			}
		}
	}()

	return chunkChan, nil
}

// ==================== 内部类型 ====================

// ChatAssistant 聊天助手信息
// 注意：RAGFlow 的 create_time/update_time 是毫秒级 Unix 时间戳（BigIntegerField），
// API 返回 JSON number，非字符串。
type ChatAssistant struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	DatasetIDs []string `json:"dataset_ids"`
	LLMModel   string   `json:"llm_model"`
	CreateTime int64    `json:"create_time"`
	UpdateTime int64    `json:"update_time"`
}

// Session 会话信息
// 注意：时间字段同样是毫秒级 Unix 时间戳。
type Session struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ChatID     string `json:"chat_id"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
}

// streamResponse SSE 流响应结构
type streamResponse struct {
	Code int `json:"code"`
	Data struct {
		Answer    string `json:"answer"`
		Reference struct {
			Chunks []Chunk `json:"chunks"`
		} `json:"reference"`
	} `json:"data"`
}
