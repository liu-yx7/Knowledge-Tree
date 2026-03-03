package ragflow_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/usememos/memos/plugin/ragflow"
)

// ==================== 测试辅助函数 ====================

func setupOpenAITestServer(handler http.HandlerFunc) (*httptest.Server, *ragflow.Client) {
	server := httptest.NewServer(handler)
	client := ragflow.NewClient(&ragflow.Config{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Timeout: 5 * time.Second,
	})
	return server, client
}

// buildSSEData 构造 SSE data 行
func buildSSEData(v any) string {
	b, _ := json.Marshal(v)
	return "data:" + string(b) + "\n\n"
}

// ==================== 非流式对话测试 ====================

func TestClient_ChatCompletion(t *testing.T) {
	stopReason := "stop"
	expectedResp := ragflow.OpenAIChatResponse{
		ID:      "chatcmpl-test-123",
		Object:  "chat.completion",
		Created: 1707000000,
		Model:   "model",
		Choices: []ragflow.OpenAIChoice{
			{
				Message: &ragflow.OpenAIResponseMessage{
					Role:    "assistant",
					Content: "Go 的并发模型基于 CSP",
					Reference: []ragflow.OpenAIReference{
						{
							ID:           "chunk_1",
							Content:      "goroutine 是轻量级线程",
							DocumentID:   "doc_1",
							DocumentName: "memo_m_abc123.txt",
							DatasetID:    "ds_1",
							Similarity:   0.85,
						},
					},
				},
				FinishReason: &stopReason,
				Index:        0,
			},
		},
		Usage: &ragflow.OpenAIUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和路径
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 方法, 实际 = %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v1/chats_openai/") {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("路径后缀不正确: %s", r.URL.Path)
		}

		// 验证 Authorization 头
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Authorization 头不正确: %s", r.Header.Get("Authorization"))
		}

		// 验证请求体
		var req ragflow.OpenAIChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Stream {
			t.Error("非流式请求的 stream 应为 false")
		}
		if len(req.Messages) == 0 {
			t.Error("messages 不应为空")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	})
	defer server.Close()

	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{
			{Role: "user", Content: "Go 的并发模型是什么？"},
		},
		false,
	))

	if err != nil {
		t.Fatalf("ChatCompletion 失败: %v", err)
	}
	if resp.ID != "chatcmpl-test-123" {
		t.Errorf("响应 ID 不匹配: %s", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("期望 1 个 choice, 实际 = %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Go 的并发模型基于 CSP" {
		t.Errorf("回答内容不匹配: %s", resp.Choices[0].Message.Content)
	}
	if len(resp.Choices[0].Message.Reference) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(resp.Choices[0].Message.Reference))
	}
	if resp.Choices[0].Message.Reference[0].DocumentName != "memo_m_abc123.txt" {
		t.Errorf("引用 document_name 不匹配")
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 150 {
		t.Errorf("Token 使用统计不正确")
	}
}

func TestClient_ChatCompletion_NoReferences(t *testing.T) {
	stopReason := "stop"
	expectedResp := ragflow.OpenAIChatResponse{
		ID:      "chatcmpl-no-ref",
		Object:  "chat.completion",
		Created: 1707000000,
		Model:   "model",
		Choices: []ragflow.OpenAIChoice{
			{
				Message: &ragflow.OpenAIResponseMessage{
					Role:    "assistant",
					Content: "抱歉，我没有找到相关信息",
				},
				FinishReason: &stopReason,
				Index:        0,
			},
		},
	}

	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResp)
	})
	defer server.Close()

	ctx := context.Background()
	resp, err := client.ChatCompletion(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{
			{Role: "user", Content: "量子力学是什么？"},
		},
		false,
	))

	if err != nil {
		t.Fatalf("ChatCompletion 失败: %v", err)
	}
	if len(resp.Choices[0].Message.Reference) != 0 {
		t.Errorf("空知识库应返回 0 个引用, 实际 = %d", len(resp.Choices[0].Message.Reference))
	}
}

func TestClient_ChatCompletion_APIError(t *testing.T) {
	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	})
	defer server.Close()

	ctx := context.Background()
	_, err := client.ChatCompletion(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{{Role: "user", Content: "test"}},
		false,
	))

	if err == nil {
		t.Fatal("期望返回错误")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误应包含状态码 401: %s", err.Error())
	}
}

// ==================== 流式对话测试 ====================

func TestClient_ChatCompletionStream(t *testing.T) {
	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("期望 Accept: text/event-stream, 实际 = %s", r.Header.Get("Accept"))
		}

		var req ragflow.OpenAIChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("流式请求的 stream 应为 true")
		}

		// 模拟 SSE 流式响应
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, _ := w.(http.Flusher)

		// chunk 1: 普通内容
		chunk1 := ragflow.OpenAIChatResponse{
			ID:    "chatcmpl-stream-1",
			Model: "model",
			Choices: []ragflow.OpenAIChoice{
				{Delta: &ragflow.OpenAIDelta{Content: strPtr("Go 的")}, Index: 0},
			},
		}
		fmt.Fprint(w, buildSSEData(chunk1))
		flusher.Flush()

		// chunk 2: 普通内容
		chunk2 := ragflow.OpenAIChatResponse{
			ID:    "chatcmpl-stream-1",
			Model: "model",
			Choices: []ragflow.OpenAIChoice{
				{Delta: &ragflow.OpenAIDelta{Content: strPtr("并发模型")}, Index: 0},
			},
		}
		fmt.Fprint(w, buildSSEData(chunk2))
		flusher.Flush()

		// chunk 3: 最后一个 chunk（含引用和 final_content）
		stopReason := "stop"
		chunk3 := ragflow.OpenAIChatResponse{
			ID:    "chatcmpl-stream-1",
			Model: "model",
			Choices: []ragflow.OpenAIChoice{
				{
					Delta: &ragflow.OpenAIDelta{
						FinalContent: "Go 的并发模型基于 CSP",
						Reference: []ragflow.OpenAIReference{
							{
								ID:           "chunk_ref_1",
								Content:      "goroutine 是轻量级协程",
								DocumentName: "memo_m_abc123.txt",
								Similarity:   0.92,
							},
						},
					},
					FinishReason: &stopReason,
					Index:        0,
				},
			},
			Usage: &ragflow.OpenAIUsage{
				PromptTokens:     80,
				CompletionTokens: 30,
				TotalTokens:      110,
			},
		}
		fmt.Fprint(w, buildSSEData(chunk3))
		flusher.Flush()

		// SSE 结束标记
		fmt.Fprint(w, "data:[DONE]\n\n")
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	chunkChan, err := client.ChatCompletionStream(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{
			{Role: "user", Content: "Go 的并发模型是什么？"},
		},
		true,
	))

	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var chunks []ragflow.OpenAIChatChunk
	for chunk := range chunkChan {
		if chunk.Error != nil {
			t.Fatalf("收到错误 chunk: %v", chunk.Error)
		}
		chunks = append(chunks, chunk)
	}

	// 验证收到 3 个 chunk
	if len(chunks) != 3 {
		t.Fatalf("期望 3 个 chunk, 实际 = %d", len(chunks))
	}

	// chunk 1: 普通内容 "Go 的"
	if chunks[0].Content != "Go 的" {
		t.Errorf("chunk 1 内容不匹配: %q", chunks[0].Content)
	}
	if chunks[0].Done {
		t.Error("chunk 1 不应标记为 Done")
	}

	// chunk 2: 普通内容 "并发模型"
	if chunks[1].Content != "并发模型" {
		t.Errorf("chunk 2 内容不匹配: %q", chunks[1].Content)
	}

	// chunk 3: 最后一个 chunk
	if !chunks[2].Done {
		t.Error("最后一个 chunk 应标记为 Done")
	}
	if chunks[2].FinishReason != "stop" {
		t.Errorf("finish_reason 应为 stop: %s", chunks[2].FinishReason)
	}
	if chunks[2].FinalContent != "Go 的并发模型基于 CSP" {
		t.Errorf("final_content 不匹配: %s", chunks[2].FinalContent)
	}
	if len(chunks[2].References) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(chunks[2].References))
	}
	if chunks[2].References[0].DocumentName != "memo_m_abc123.txt" {
		t.Errorf("引用 document_name 不匹配")
	}
	if chunks[2].Usage == nil || chunks[2].Usage.TotalTokens != 110 {
		t.Errorf("Token 使用统计不正确")
	}
}

func TestClient_ChatCompletionStream_WithReasoningContent(t *testing.T) {
	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// chunk 含 reasoning_content（DeepSeek 风格思考链）
		chunk1 := ragflow.OpenAIChatResponse{
			ID: "chatcmpl-reason-1",
			Choices: []ragflow.OpenAIChoice{
				{Delta: &ragflow.OpenAIDelta{
					Content:          strPtr("回答"),
					ReasoningContent: strPtr("让我思考一下..."),
				}, Index: 0},
			},
		}
		fmt.Fprint(w, buildSSEData(chunk1))
		flusher.Flush()

		stopReason := "stop"
		chunkFinal := ragflow.OpenAIChatResponse{
			ID: "chatcmpl-reason-1",
			Choices: []ragflow.OpenAIChoice{
				{
					Delta:        &ragflow.OpenAIDelta{FinalContent: "回答"},
					FinishReason: &stopReason,
					Index:        0,
				},
			},
		}
		fmt.Fprint(w, buildSSEData(chunkFinal))
		flusher.Flush()
		fmt.Fprint(w, "data:[DONE]\n\n")
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	chunkChan, err := client.ChatCompletionStream(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{{Role: "user", Content: "test"}},
		true,
	))
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var chunks []ragflow.OpenAIChatChunk
	for chunk := range chunkChan {
		if chunk.Error != nil {
			t.Fatalf("收到错误 chunk: %v", chunk.Error)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) < 1 {
		t.Fatal("期望至少 1 个 chunk")
	}
	if chunks[0].ReasoningContent != "让我思考一下..." {
		t.Errorf("reasoning_content 不匹配: %q", chunks[0].ReasoningContent)
	}
}

func TestClient_ChatCompletionStream_APIError(t *testing.T) {
	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	})
	defer server.Close()

	ctx := context.Background()
	_, err := client.ChatCompletionStream(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{{Role: "user", Content: "test"}},
		true,
	))

	if err == nil {
		t.Fatal("期望返回错误")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("错误应包含状态码 500: %s", err.Error())
	}
}

func TestClient_ChatCompletionStream_ContextCancel(t *testing.T) {
	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// 发送一个 chunk 然后等待（模拟慢响应）
		chunk := ragflow.OpenAIChatResponse{
			ID: "chatcmpl-slow",
			Choices: []ragflow.OpenAIChoice{
				{Delta: &ragflow.OpenAIDelta{Content: strPtr("开始")}, Index: 0},
			},
		}
		fmt.Fprint(w, buildSSEData(chunk))
		flusher.Flush()

		// 等足够长的时间让 context 取消
		time.Sleep(2 * time.Second)
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	chunkChan, err := client.ChatCompletionStream(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{{Role: "user", Content: "test"}},
		true,
	))
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var receivedChunks int
	for chunk := range chunkChan {
		receivedChunks++
		_ = chunk
	}

	// 应收到至少 1 个 chunk（正常内容或取消/错误）
	if receivedChunks == 0 {
		t.Error("期望至少收到 1 个 chunk")
	}
}

func TestClient_ChatCompletionStream_EmptyStream(t *testing.T) {
	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// 只发 [DONE]，不发任何内容 chunk
		fmt.Fprint(w, "data:[DONE]\n\n")
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	chunkChan, err := client.ChatCompletionStream(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{{Role: "user", Content: "test"}},
		true,
	))
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var chunks []ragflow.OpenAIChatChunk
	for chunk := range chunkChan {
		chunks = append(chunks, chunk)
	}

	// 空流：channel 应正常关闭，不应 panic
	if len(chunks) != 0 {
		t.Errorf("空流应返回 0 个 chunk, 实际 = %d", len(chunks))
	}
}

// ==================== 类型辅助函数测试 ====================

func TestNewOpenAIChatRequest(t *testing.T) {
	msgs := []ragflow.OpenAIMessage{
		{Role: "user", Content: "hello"},
	}
	req := ragflow.NewOpenAIChatRequest(msgs, true)

	if req.Model != "model" {
		t.Errorf("默认 model 不正确: %s", req.Model)
	}
	if !req.Stream {
		t.Error("stream 应为 true")
	}
	if req.ExtraBody == nil || !req.ExtraBody.Reference {
		t.Error("ExtraBody.Reference 应为 true")
	}
	if len(req.Messages) != 1 {
		t.Errorf("messages 长度不正确: %d", len(req.Messages))
	}
}

// ==================== SSE 格式兼容性测试 ====================

func TestClient_ChatCompletionStream_SSEFormatVariants(t *testing.T) {
	// 测试 "data:" 和 "data: " 两种 SSE 格式
	server, client := setupOpenAITestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// 格式 1: "data:" 无空格
		chunk1 := `data:{"id":"test","choices":[{"delta":{"content":"A"},"index":0}]}` + "\n\n"
		fmt.Fprint(w, chunk1)
		flusher.Flush()

		// 格式 2: "data: " 有空格
		chunk2 := `data: {"id":"test","choices":[{"delta":{"content":"B"},"index":0}]}` + "\n\n"
		fmt.Fprint(w, chunk2)
		flusher.Flush()

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	})
	defer server.Close()

	ctx := context.Background()
	chunkChan, err := client.ChatCompletionStream(ctx, "assistant-1", ragflow.NewOpenAIChatRequest(
		[]ragflow.OpenAIMessage{{Role: "user", Content: "test"}},
		true,
	))
	if err != nil {
		t.Fatalf("ChatCompletionStream 失败: %v", err)
	}

	var contents []string
	for chunk := range chunkChan {
		if chunk.Error != nil {
			t.Fatalf("收到错误 chunk: %v", chunk.Error)
		}
		if chunk.Content != "" {
			contents = append(contents, chunk.Content)
		}
	}

	if len(contents) != 2 {
		t.Fatalf("期望 2 个内容 chunk, 实际 = %d", len(contents))
	}
	if contents[0] != "A" || contents[1] != "B" {
		t.Errorf("内容不匹配: %v", contents)
	}
}

// ==================== 辅助函数 ====================

func strPtr(s string) *string {
	return &s
}
