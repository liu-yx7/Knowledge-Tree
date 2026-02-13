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
