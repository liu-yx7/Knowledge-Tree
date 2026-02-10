package ragflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ==================== 测试辅助函数 ====================

// mockOpenAIServer 创建模拟 RAGFlow OpenAI 兼容 API 的测试服务器
func mockOpenAIServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)
	client := NewClient(&Config{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Timeout: 10 * time.Second,
	})
	return server, client
}

// buildSSELine 构建一行 SSE data
func buildSSELine(data string) string {
	return "data:" + data + "\n\n"
}

// buildSSEChunk 构建标准的流式 chunk SSE 行
func buildSSEChunk(content string) string {
	chunk := OpenAIChatResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Created: 1707000000,
		Model:   "model",
		Choices: []OpenAIChoice{
			{
				Delta: &OpenAIDelta{
					Content: stringPtr(content),
					Role:    "assistant",
				},
				Index: 0,
			},
		},
	}
	data, _ := json.Marshal(chunk)
	return buildSSELine(string(data))
}

// buildSSEFinalChunk 构建最后一个 chunk（含引用和 final_content）
func buildSSEFinalChunk(finalContent string, refs []OpenAIReference, usage *OpenAIUsage) string {
	stop := "stop"
	chunk := OpenAIChatResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Created: 1707000000,
		Model:   "model",
		Choices: []OpenAIChoice{
			{
				Delta: &OpenAIDelta{
					Reference:    refs,
					FinalContent: finalContent,
				},
				FinishReason: &stop,
				Index:        0,
			},
		},
		Usage: usage,
	}
	data, _ := json.Marshal(chunk)
	return buildSSELine(string(data))
}

// ==================== 非流式调用测试 ====================

func TestChatCompletion_Success(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		if !strings.Contains(r.URL.Path, "/api/v1/chats_openai/") {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}
		// 验证 Authorization header
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Authorization 不正确: %s", r.Header.Get("Authorization"))
		}
		// 验证请求体
		var req OpenAIChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		if req.Stream {
			t.Error("非流式调用不应设置 stream=true")
		}
		if len(req.Messages) != 1 {
			t.Errorf("消息数量不正确: %d", len(req.Messages))
		}

		// 返回非流式响应
		resp := OpenAIChatResponse{
			ID:      "chatcmpl-abc123",
			Object:  "chat.completion",
			Created: 1707000000,
			Model:   "model",
			Choices: []OpenAIChoice{
				{
					Message: &OpenAIResponseMessage{
						Role:    "assistant",
						Content: "这是回答",
						Reference: []OpenAIReference{
							{
								ID:           "chunk-001",
								Content:      "匹配的内容",
								DocumentID:   "doc-001",
								DocumentName: "memo_m_abc123.txt",
								DatasetID:    "ds-001",
								Similarity:   0.85,
							},
						},
					},
					FinishReason: stringPtr("stop"),
					Index:        0,
				},
			},
			Usage: &OpenAIUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{
			{Role: "user", Content: "你好"},
		},
	}

	resp, err := client.ChatCompletion(ctx, "assistant-id-001", req)
	if err != nil {
		t.Fatalf("ChatCompletion 失败: %v", err)
	}

	if resp.ID != "chatcmpl-abc123" {
		t.Errorf("响应 ID 不正确: %s", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("Choices 数量不正确: %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "这是回答" {
		t.Errorf("回答内容不正确: %s", resp.Choices[0].Message.Content)
	}
	if len(resp.Choices[0].Message.Reference) != 1 {
		t.Fatalf("引用数量不正确: %d", len(resp.Choices[0].Message.Reference))
	}
	if resp.Choices[0].Message.Reference[0].DocumentName != "memo_m_abc123.txt" {
		t.Errorf("引用文档名不正确: %s", resp.Choices[0].Message.Reference[0].DocumentName)
	}
	if resp.Usage.TotalTokens != 150 {
		t.Errorf("Token 统计不正确: %d", resp.Usage.TotalTokens)
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	_, err := client.ChatCompletion(ctx, "assistant-id", req)
	if err == nil {
		t.Fatal("HTTP 500 应该返回错误")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("错误信息应包含状态码: %v", err)
	}
}

func TestChatCompletion_InvalidJSON(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	_, err := client.ChatCompletion(ctx, "assistant-id", req)
	if err == nil {
		t.Fatal("无效 JSON 应该返回错误")
	}
}

func TestChatCompletion_RequestPath(t *testing.T) {
	var capturedPath string
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		resp := OpenAIChatResponse{Choices: []OpenAIChoice{}}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	client.ChatCompletion(ctx, "my-assistant-123", req)

	expected := "/api/v1/chats_openai/my-assistant-123/chat/completions"
	if capturedPath != expected {
		t.Errorf("路径不正确:\n  期望: %s\n  实际: %s", expected, capturedPath)
	}
}

// ==================== 流式调用测试 ====================

func TestChatCompletionStream_Success(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求体中 stream=true
		var req OpenAIChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("流式调用应设置 stream=true")
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Error("流式调用应设置 Accept: text/event-stream")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// 发送 3 个普通 chunk
		fmt.Fprint(w, buildSSEChunk("你"))
		flusher.Flush()
		fmt.Fprint(w, buildSSEChunk("好"))
		flusher.Flush()
		fmt.Fprint(w, buildSSEChunk("世界"))
		flusher.Flush()

		// 发送最后一个 chunk（含引用）
		refs := []OpenAIReference{
			{
				ID:           "chunk-001",
				Content:      "相关内容片段",
				DocumentName: "memo_m_test123.txt",
				Similarity:   0.92,
			},
		}
		usage := &OpenAIUsage{
			PromptTokens:     80,
			CompletionTokens: 30,
			TotalTokens:      110,
		}
		fmt.Fprint(w, buildSSEFinalChunk("你好世界", refs, usage))
		flusher.Flush()

		// 发送 [DONE]
		fmt.Fprint(w, buildSSELine("[DONE]"))
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "你好"}},
		ExtraBody: &OpenAIExtraBody{
			Reference: true,
		},
	}

	chunks, err := client.ChatCompletionStream(ctx, "assistant-id", req)
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var contents []string
	var finalChunk OpenAIChatChunk

	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("收到错误 chunk: %v", chunk.Error)
		}
		if chunk.Done {
			finalChunk = chunk
			break
		}
		contents = append(contents, chunk.Content)
	}

	// 验证内容 chunks
	if len(contents) != 3 {
		t.Fatalf("内容 chunk 数量不正确: %d, 内容: %v", len(contents), contents)
	}
	if contents[0] != "你" || contents[1] != "好" || contents[2] != "世界" {
		t.Errorf("内容不正确: %v", contents)
	}

	// 验证最后一个 chunk
	if !finalChunk.Done {
		t.Error("最后一个 chunk 应该标记为 Done")
	}
	if finalChunk.FinishReason != "stop" {
		t.Errorf("FinishReason 不正确: %s", finalChunk.FinishReason)
	}
	if finalChunk.FinalContent != "你好世界" {
		t.Errorf("FinalContent 不正确: %s", finalChunk.FinalContent)
	}
	if len(finalChunk.References) != 1 {
		t.Fatalf("引用数量不正确: %d", len(finalChunk.References))
	}
	if finalChunk.References[0].DocumentName != "memo_m_test123.txt" {
		t.Errorf("引用文档名不正确: %s", finalChunk.References[0].DocumentName)
	}
	if finalChunk.Usage == nil {
		t.Fatal("Usage 应该非空")
	}
	if finalChunk.Usage.TotalTokens != 110 {
		t.Errorf("TotalTokens 不正确: %d", finalChunk.Usage.TotalTokens)
	}
}

func TestChatCompletionStream_HTTPError(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	_, err := client.ChatCompletionStream(ctx, "assistant-id", req)
	if err == nil {
		t.Fatal("HTTP 401 应该返回错误")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误信息应包含状态码: %v", err)
	}
}

func TestChatCompletionStream_ContextCancelled(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// 发送一个 chunk 然后等待（模拟慢响应）
		fmt.Fprint(w, buildSSEChunk("第一个"))
		flusher.Flush()

		// 等足够长时间让 context 被取消
		time.Sleep(2 * time.Second)
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	chunks, err := client.ChatCompletionStream(ctx, "assistant-id", req)
	if err != nil {
		t.Fatalf("ChatCompletionStream 不应在创建时失败: %v", err)
	}

	receivedContent := false
	receivedError := false
	for chunk := range chunks {
		if chunk.Content == "第一个" {
			receivedContent = true
		}
		if chunk.Error != nil {
			receivedError = true
		}
	}

	// 至少收到了一个内容 chunk 或者一个错误
	if !receivedContent && !receivedError {
		t.Error("应该至少收到内容或错误")
	}
}

func TestChatCompletionStream_EmptyStream(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// 只发送 [DONE]
		fmt.Fprint(w, buildSSELine("[DONE]"))
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	chunks, err := client.ChatCompletionStream(ctx, "assistant-id", req)
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	count := 0
	for range chunks {
		count++
	}

	// [DONE] 不产生 chunk，channel 直接关闭
	if count != 0 {
		t.Errorf("空流应该不产生 chunk，实际: %d", count)
	}
}

func TestChatCompletionStream_WithReasoningContent(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// 发送含 reasoning_content 的 chunk
		thinkChunk := OpenAIChatResponse{
			ID:    "chatcmpl-test",
			Model: "model",
			Choices: []OpenAIChoice{
				{
					Delta: &OpenAIDelta{
						Content:          stringPtr(""),
						ReasoningContent: stringPtr("让我思考一下..."),
					},
					Index: 0,
				},
			},
		}
		data, _ := json.Marshal(thinkChunk)
		fmt.Fprint(w, buildSSELine(string(data)))
		flusher.Flush()

		// 发送正常内容
		fmt.Fprint(w, buildSSEChunk("回答内容"))
		flusher.Flush()

		// 结束
		fmt.Fprint(w, buildSSEFinalChunk("回答内容", nil, nil))
		flusher.Flush()
		fmt.Fprint(w, buildSSELine("[DONE]"))
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	chunks, err := client.ChatCompletionStream(ctx, "assistant-id", req)
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var gotReasoning bool
	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("收到错误: %v", chunk.Error)
		}
		if chunk.ReasoningContent == "让我思考一下..." {
			gotReasoning = true
		}
		if chunk.Done {
			break
		}
	}

	if !gotReasoning {
		t.Error("未收到 reasoning_content")
	}
}

func TestChatCompletionStream_NoReferences(t *testing.T) {
	server, client := mockOpenAIServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		fmt.Fprint(w, buildSSEChunk("回答"))
		flusher.Flush()

		// 最后一个 chunk 无引用
		fmt.Fprint(w, buildSSEFinalChunk("回答", nil, nil))
		flusher.Flush()
		fmt.Fprint(w, buildSSELine("[DONE]"))
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	req := &OpenAIChatCompletionRequest{
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	chunks, err := client.ChatCompletionStream(ctx, "assistant-id", req)
	if err != nil {
		t.Fatalf("失败: %v", err)
	}

	for chunk := range chunks {
		if chunk.Done {
			if len(chunk.References) != 0 {
				t.Errorf("无引用时 References 应为空: %d", len(chunk.References))
			}
			break
		}
	}
}

// ==================== SSE 解析器单元测试 ====================

func TestParseSSEChunk_NormalChunk(t *testing.T) {
	data := `{"id":"chatcmpl-test","choices":[{"delta":{"content":"hello","role":"assistant"},"index":0}],"model":"model"}`

	chunk := parseSSEChunk(data)

	if chunk.Error != nil {
		t.Fatalf("不应有错误: %v", chunk.Error)
	}
	if chunk.Content != "hello" {
		t.Errorf("Content 不正确: %s", chunk.Content)
	}
	if chunk.Done {
		t.Error("普通 chunk 不应标记为 Done")
	}
}

func TestParseSSEChunk_FinalChunk(t *testing.T) {
	data := `{"id":"chatcmpl-test","choices":[{"delta":{"content":null,"reference":[{"id":"c1","document_name":"memo_m_abc.txt","similarity":0.9}],"final_content":"完整回答"},"finish_reason":"stop","index":0}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`

	chunk := parseSSEChunk(data)

	if chunk.Error != nil {
		t.Fatalf("不应有错误: %v", chunk.Error)
	}
	if !chunk.Done {
		t.Error("最后一个 chunk 应标记为 Done")
	}
	if chunk.FinishReason != "stop" {
		t.Errorf("FinishReason 不正确: %s", chunk.FinishReason)
	}
	if chunk.FinalContent != "完整回答" {
		t.Errorf("FinalContent 不正确: %s", chunk.FinalContent)
	}
	if len(chunk.References) != 1 {
		t.Fatalf("引用数量不正确: %d", len(chunk.References))
	}
	if chunk.References[0].Similarity != 0.9 {
		t.Errorf("相似度不正确: %f", chunk.References[0].Similarity)
	}
	if chunk.Usage == nil || chunk.Usage.TotalTokens != 15 {
		t.Errorf("Usage 不正确: %+v", chunk.Usage)
	}
}

func TestParseSSEChunk_InvalidJSON(t *testing.T) {
	chunk := parseSSEChunk("{invalid json}")

	if chunk.Error == nil {
		t.Fatal("无效 JSON 应该返回错误")
	}
}

func TestParseSSEChunk_EmptyChoices(t *testing.T) {
	chunk := parseSSEChunk(`{"id":"test","choices":[],"model":"model"}`)

	if chunk.Error == nil {
		t.Fatal("空 choices 应该返回错误")
	}
}

func TestParseSSEChunk_NilDelta(t *testing.T) {
	data := `{"id":"test","choices":[{"index":0}],"model":"model"}`

	chunk := parseSSEChunk(data)

	// delta 为 nil 时不应 panic，Content 应为空
	if chunk.Error != nil {
		t.Fatalf("不应有错误: %v", chunk.Error)
	}
	if chunk.Content != "" {
		t.Errorf("Content 应为空: %s", chunk.Content)
	}
}

// ==================== 辅助函数测试 ====================

func TestDerefString(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{"nil 指针", nil, ""},
		{"空字符串", stringPtr(""), ""},
		{"正常字符串", stringPtr("hello"), "hello"},
		{"中文字符串", stringPtr("你好"), "你好"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := derefString(tt.input)
			if result != tt.expected {
				t.Errorf("期望 %q, 实际 %q", tt.expected, result)
			}
		})
	}
}

func TestStringPtr(t *testing.T) {
	s := "test"
	ptr := stringPtr(s)
	if ptr == nil {
		t.Fatal("stringPtr 返回 nil")
	}
	if *ptr != "test" {
		t.Errorf("值不正确: %s", *ptr)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"短字符串", "hello", 10, "hello"},
		{"精确长度", "hello", 5, "hello"},
		{"截断", "hello world", 5, "hello..."},
		{"空字符串", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("期望 %q, 实际 %q", tt.expected, result)
			}
		})
	}
}

func TestNewOpenAIChatRequest(t *testing.T) {
	messages := []OpenAIMessage{
		{Role: "user", Content: "你好"},
	}

	// 测试流式
	req := NewOpenAIChatRequest(messages, true)
	if req.Model != "model" {
		t.Errorf("Model 不正确: %s", req.Model)
	}
	if !req.Stream {
		t.Error("Stream 应为 true")
	}
	if req.ExtraBody == nil || !req.ExtraBody.Reference {
		t.Error("ExtraBody.Reference 应为 true")
	}
	if len(req.Messages) != 1 {
		t.Errorf("Messages 数量不正确: %d", len(req.Messages))
	}

	// 测试非流式
	req2 := NewOpenAIChatRequest(messages, false)
	if req2.Stream {
		t.Error("Stream 应为 false")
	}
}

func TestOpenaiChatCompletionPath(t *testing.T) {
	path := openaiChatCompletionPath("my-chat-id-123")
	expected := "/api/v1/chats_openai/my-chat-id-123/chat/completions"
	if path != expected {
		t.Errorf("路径不正确:\n  期望: %s\n  实际: %s", expected, path)
	}
}

// ==================== ExtraBody 序列化测试 ====================

func TestExtraBody_JSONSerialization(t *testing.T) {
	req := &OpenAIChatCompletionRequest{
		Model: "model",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "test"},
		},
		Stream: true,
		ExtraBody: &OpenAIExtraBody{
			Reference: true,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"extra_body"`) {
		t.Error("JSON 应包含 extra_body 字段")
	}
	if !strings.Contains(jsonStr, `"reference":true`) {
		t.Error("JSON 应包含 reference:true")
	}
}

func TestExtraBody_OmitEmpty(t *testing.T) {
	req := &OpenAIChatCompletionRequest{
		Model:    "model",
		Messages: []OpenAIMessage{{Role: "user", Content: "test"}},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, "extra_body") {
		t.Error("ExtraBody 为 nil 时不应序列化 extra_body 字段")
	}
}
