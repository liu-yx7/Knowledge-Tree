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

// ==================== OpenAI 兼容 API 路径 ====================

// openaiChatCompletionPath 构建 OpenAI 兼容 API 的请求路径
// chatID 是 RAGFlow Chat Assistant ID（由 Provisioner 创建）
func openaiChatCompletionPath(chatID string) string {
	return fmt.Sprintf("/api/v1/chats_openai/%s/chat/completions", chatID)
}

// ==================== 非流式对话 ====================

// ChatCompletion 非流式对话（使用 OpenAI 兼容 API）
// chatID: RAGFlow Chat Assistant ID（Provisioner.EnsureUserResources 返回的 AssistantID）
// 返回完整响应，包含引用信息
func (c *Client) ChatCompletion(ctx context.Context, chatID string, req *OpenAIChatCompletionRequest) (*OpenAIChatResponse, error) {
	req.Stream = false

	resp, err := c.request(ctx, http.MethodPost, openaiChatCompletionPath(chatID), req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI 兼容 API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("OpenAI API 错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp OpenAIChatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 响应失败: %w", err)
	}

	return &chatResp, nil
}

// ==================== 流式对话 ====================

// ChatCompletionStream 流式对话（使用 OpenAI 兼容 API）
// chatID: RAGFlow Chat Assistant ID
// 返回 channel 逐个接收 chunk，最后一个 chunk（Done=true）含引用信息和 FinalContent
//
// 消费示例:
//
//	chunks, _ := client.ChatCompletionStream(ctx, assistantID, req)
//	var fullContent string
//	for chunk := range chunks {
//	    if chunk.Error != nil { handleError(chunk.Error); break }
//	    if chunk.Done { /* 提取 chunk.References, chunk.FinalContent */ break }
//	    fullContent += chunk.Content
//	}
func (c *Client) ChatCompletionStream(ctx context.Context, chatID string, req *OpenAIChatCompletionRequest) (<-chan OpenAIChatChunk, error) {
	req.Stream = true

	url := c.config.BaseURL + openaiChatCompletionPath(chatID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if c.HasAPIKey() {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送流式请求失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API 错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	chunkChan := make(chan OpenAIChatChunk, 100)

	go func() {
		defer close(chunkChan)
		defer resp.Body.Close()
		parseSSEStream(ctx, resp.Body, chunkChan)
	}()

	return chunkChan, nil
}

// ==================== SSE 流式解析器 ====================

// parseSSEStream 解析标准 SSE 事件流
// 协议格式:
//
//	data:{"id":"chatcmpl-xxx","choices":[{"delta":{"content":"片段"}}]}\n
//	\n
//	data:[DONE]\n
//
// 最后一个 chunk（finish_reason=stop）含 delta.reference 和 delta.final_content
func parseSSEStream(ctx context.Context, body io.Reader, chunkChan chan<- OpenAIChatChunk) {
	reader := bufio.NewReader(body)

	for {
		// 检查 context 取消
		select {
		case <-ctx.Done():
			chunkChan <- OpenAIChatChunk{
				Done:         true,
				FinishReason: "cancelled",
				Error:        ctx.Err(),
			}
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// 流正常结束（可能 RAGFlow 未发 [DONE] 就关闭了连接）
				return
			}
			chunkChan <- OpenAIChatChunk{
				Done:         true,
				FinishReason: "error",
				Error:        fmt.Errorf("读取 SSE 流失败: %w", err),
			}
			return
		}

		line = strings.TrimSpace(line)

		// 跳过空行（SSE 事件分隔符）和非 data 行
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		// 提取 data 内容（兼容 "data:" 和 "data: " 两种格式）
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		// 检查流结束标记
		if data == "[DONE]" {
			return
		}

		// 解析 JSON
		chunk := parseSSEChunk(data)
		chunkChan <- chunk

		// 如果是最后一个 chunk，结束循环
		if chunk.Done {
			return
		}
	}
}

// parseSSEChunk 解析单个 SSE data 为 OpenAIChatChunk
func parseSSEChunk(data string) OpenAIChatChunk {
	var resp OpenAIChatResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return OpenAIChatChunk{
			Error: fmt.Errorf("解析 SSE chunk 失败: %w (data: %s)", err, truncate(data, 200)),
		}
	}

	if len(resp.Choices) == 0 {
		return OpenAIChatChunk{
			Error: fmt.Errorf("SSE chunk 中 choices 为空"),
		}
	}

	choice := resp.Choices[0]

	// 判断是否是最后一个 chunk（finish_reason 非空）
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		chunk := OpenAIChatChunk{
			Done:         true,
			FinishReason: *choice.FinishReason,
			Usage:        resp.Usage,
		}

		// 最后一个 chunk 的 delta 中含引用和完整回答
		if choice.Delta != nil {
			chunk.References = choice.Delta.Reference
			chunk.FinalContent = choice.Delta.FinalContent
			chunk.Content = derefString(choice.Delta.Content)
			chunk.ReasoningContent = derefString(choice.Delta.ReasoningContent)
		}

		return chunk
	}

	// 普通 chunk：提取增量文本
	chunk := OpenAIChatChunk{}
	if choice.Delta != nil {
		chunk.Content = derefString(choice.Delta.Content)
		chunk.ReasoningContent = derefString(choice.Delta.ReasoningContent)
	}

	return chunk
}

// truncate 截断字符串，用于错误日志（避免打印大量数据）
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
