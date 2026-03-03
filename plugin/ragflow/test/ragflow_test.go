package ragflow_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/usememos/memos/plugin/ragflow"
)

// ==================== 测试辅助函数 ====================

func setupTestServer(handler http.HandlerFunc) (*httptest.Server, *ragflow.Client) {
	server := httptest.NewServer(handler)
	client := ragflow.NewClient(&ragflow.Config{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Timeout: 5 * time.Second,
	})
	return server, client
}

func jsonResponse(code int, data any) []byte {
	resp := map[string]any{
		"code":    code,
		"data":    data,
		"message": "",
	}
	b, _ := json.Marshal(resp)
	return b
}

// ==================== Config 测试 ====================

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ragflow.Config
		wantErr bool
	}{
		{
			name: "有效配置（含 APIKey）",
			config: &ragflow.Config{
				BaseURL: "http://localhost:9380",
				APIKey:  "test-key",
			},
			wantErr: false,
		},
		{
			name: "有效配置（不含 APIKey，per-user 模式）",
			config: &ragflow.Config{
				BaseURL: "http://localhost:9380",
			},
			wantErr: false,
		},
		{
			name: "缺少 BaseURL",
			config: &ragflow.Config{
				APIKey: "test-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_WithDefaults(t *testing.T) {
	cfg := &ragflow.Config{
		BaseURL: "http://localhost:9380",
	}

	cfg.WithDefaults()

	if cfg.Timeout != 30*time.Second {
		t.Errorf("期望 Timeout = 30s, 实际 = %v", cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("期望 MaxRetries = 3, 实际 = %d", cfg.MaxRetries)
	}
}

// ==================== Client 测试 ====================

func TestNewClient(t *testing.T) {
	cfg := &ragflow.Config{
		BaseURL: "http://localhost:9380",
		APIKey:  "test-key",
	}

	client := ragflow.NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient 返回 nil")
	}
	if !client.HasAPIKey() {
		t.Error("期望 HasAPIKey() 为 true")
	}
}

func TestNewClient_WithoutAPIKey(t *testing.T) {
	cfg := &ragflow.Config{
		BaseURL: "http://localhost:9380",
	}

	client := ragflow.NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient 返回 nil（无 APIKey 也应能创建）")
	}
	if client.HasAPIKey() {
		t.Error("期望 HasAPIKey() 为 false")
	}
}

func TestClient_WithAPIKey(t *testing.T) {
	cfg := &ragflow.Config{
		BaseURL: "http://localhost:9380",
	}
	client := ragflow.NewClient(cfg)

	if client.HasAPIKey() {
		t.Fatal("初始客户端不应有 APIKey")
	}

	userClient := client.WithAPIKey("user-api-key")
	if !userClient.HasAPIKey() {
		t.Error("WithAPIKey 返回的客户端应有 APIKey")
	}
	// 原始客户端不受影响
	if client.HasAPIKey() {
		t.Error("原始客户端不应被修改")
	}
}

// ==================== 数据集操作测试 ====================

func TestClient_ListDatasets(t *testing.T) {
	datasets := []ragflow.Dataset{
		{ID: "ds1", Name: "测试数据集1", ChunkMethod: "naive"},
		{ID: "ds2", Name: "测试数据集2", ChunkMethod: "qa"},
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET 方法, 实际 = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Authorization 头不正确")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, datasets))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.ListDatasets(ctx, &ragflow.ListOptions{Page: 1, PageSize: 10})

	if err != nil {
		t.Fatalf("ListDatasets 失败: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("期望 2 个数据集, 实际 = %d", len(result))
	}
	if result[0].ID != "ds1" {
		t.Errorf("第一个数据集 ID 不正确")
	}
}

func TestClient_CreateDataset(t *testing.T) {
	expectedDataset := &ragflow.Dataset{
		ID:          "new-ds",
		Name:        "新数据集",
		Description: "测试描述",
		ChunkMethod: "naive",
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 方法, 实际 = %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedDataset))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.CreateDataset(ctx, &ragflow.CreateDatasetRequest{
		Name:        "新数据集",
		Description: "测试描述",
		ChunkMethod: ragflow.ChunkMethodNaive,
	})

	if err != nil {
		t.Fatalf("CreateDataset 失败: %v", err)
	}
	if result.ID != expectedDataset.ID {
		t.Errorf("数据集 ID 不匹配")
	}
}

func TestClient_DeleteDataset(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("期望 DELETE 方法, 实际 = %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, nil))
	})
	defer server.Close()

	ctx := context.Background()
	err := client.DeleteDataset(ctx, "ds-to-delete")

	if err != nil {
		t.Fatalf("DeleteDataset 失败: %v", err)
	}
}

// ==================== 检索测试 ====================

func TestClient_Retrieve(t *testing.T) {
	expectedResult := &ragflow.RetrievalResult{
		Chunks: []ragflow.Chunk{
			{ID: "c1", Content: "测试内容1", Similarity: 0.95},
			{ID: "c2", Content: "测试内容2", Similarity: 0.85},
		},
		Total: 2,
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 方法, 实际 = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/retrieval" {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedResult))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.Retrieve(ctx, &ragflow.RetrievalRequest{
		DatasetIDs: []string{"ds1"},
		Question:   "测试问题",
		TopK:       5,
	})

	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if len(result.Chunks) != 2 {
		t.Errorf("期望 2 个块, 实际 = %d", len(result.Chunks))
	}
	if result.Chunks[0].Similarity != 0.95 {
		t.Errorf("相似度不正确")
	}
}

// ==================== 会话测试 ====================

func TestClient_CreateSession(t *testing.T) {
	expectedSession := &ragflow.Session{
		ID:     "session-1",
		Name:   "测试会话",
		ChatID: "chat-1",
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 方法, 实际 = %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedSession))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.CreateSession(ctx, "chat-1", "测试会话")

	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	if result.ID != expectedSession.ID {
		t.Errorf("会话 ID 不匹配")
	}
}

func TestClient_Chat(t *testing.T) {
	expectedResponse := &ragflow.ChatResponse{
		Answer: "这是 AI 的回答",
		References: []ragflow.Chunk{
			{ID: "ref1", Content: "参考内容"},
		},
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 方法, 实际 = %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedResponse))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.Chat(ctx, "chat-1", &ragflow.ChatRequest{
		SessionID: "session-1",
		Question:  "你好",
		Stream:    false,
	})

	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if result.Answer != expectedResponse.Answer {
		t.Errorf("回答不匹配")
	}
}

// ==================== 健康检查测试 ====================

func TestClient_HealthCheck(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, []ragflow.Dataset{}))
	})
	defer server.Close()

	ctx := context.Background()
	err := client.HealthCheck(ctx)

	if err != nil {
		t.Fatalf("HealthCheck 失败: %v", err)
	}
}

// ==================== 错误处理测试 ====================

func TestClient_APIError(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code": 401, "message": "Unauthorized"}`))
	})
	defer server.Close()

	ctx := context.Background()
	_, err := client.ListDatasets(ctx, ragflow.DefaultListOptions())

	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestClient_RAGFlowError(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"code":    1001,
			"data":    nil,
			"message": "数据集不存在",
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	})
	defer server.Close()

	ctx := context.Background()
	_, err := client.ListDatasets(ctx, ragflow.DefaultListOptions())

	if err == nil {
		t.Fatal("期望返回 RAGFlow 错误")
	}
}

// TestClient_RAGFlowAuthError 测试 RAGFlow 认证错误时 data 字段为 bool 的情况
func TestClient_RAGFlowAuthError(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 模拟 RAGFlow 认证失败：data 字段返回 false（bool）
		w.Write([]byte(`{"code":109,"data":false,"message":"Authentication error: API key is invalid!"}`))
	})
	defer server.Close()

	ctx := context.Background()
	_, err := client.ListDatasets(ctx, ragflow.DefaultListOptions())

	if err == nil {
		t.Fatal("期望返回认证错误")
	}
	// 错误信息应包含业务错误码和消息，而非 json unmarshal 错误
	if !contains(err.Error(), "code=109") {
		t.Errorf("错误信息应包含业务错误码，实际: %s", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ==================== 类型测试 ====================

func TestNewTextDocument(t *testing.T) {
	doc := ragflow.NewTextDocument("test", "内容")

	if doc.Name != "test.txt" {
		t.Errorf("文档名不正确: %s", doc.Name)
	}
	if doc.MimeType != "text/plain" {
		t.Errorf("MIME 类型不正确: %s", doc.MimeType)
	}
	if string(doc.Content) != "内容" {
		t.Errorf("内容不正确")
	}
}

func TestDefaultRetrievalRequest(t *testing.T) {
	req := ragflow.DefaultRetrievalRequest([]string{"ds1"}, "问题")

	if req.TopK != 6 {
		t.Errorf("默认 TopK 不正确: %d", req.TopK)
	}
	if req.SimilarityThreshold != 0.1 {
		t.Errorf("默认 SimilarityThreshold 不正确: %f", req.SimilarityThreshold)
	}
	if req.KeywordWeight != 0.3 {
		t.Errorf("默认 KeywordWeight 不正确: %f", req.KeywordWeight)
	}
}

// ==================== 文档操作测试 ====================

func TestClient_GetDocument(t *testing.T) {
	expectedDoc := &ragflow.DocumentInfo{
		ID:         "doc-1",
		Name:       "test.txt",
		Size:       1024,
		ChunkCount: 5,
		Status:     "completed",
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET 方法, 实际 = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/datasets/ds-1/documents/doc-1" {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedDoc))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.GetDocument(ctx, "ds-1", "doc-1")

	if err != nil {
		t.Fatalf("GetDocument 失败: %v", err)
	}
	if result.ID != expectedDoc.ID {
		t.Errorf("文档 ID 不匹配")
	}
	if result.Status != "completed" {
		t.Errorf("文档状态不正确: %s", result.Status)
	}
}

func TestClient_UpdateDocument(t *testing.T) {
	expectedDoc := &ragflow.DocumentInfo{
		ID:       "doc-1",
		Name:     "updated.txt",
		Metadata: map[string]any{"visibility": "PUBLIC"},
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("期望 PUT 方法, 实际 = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/datasets/ds-1/documents/doc-1" {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}

		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["name"] != "updated.txt" {
			t.Errorf("请求体 name 不正确: %v", payload["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedDoc))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.UpdateDocument(ctx, "ds-1", "doc-1", &ragflow.UpdateDocumentRequest{
		Name:     "updated.txt",
		Metadata: map[string]any{"visibility": "PUBLIC"},
	})

	if err != nil {
		t.Fatalf("UpdateDocument 失败: %v", err)
	}
	if result.Name != "updated.txt" {
		t.Errorf("文档名不匹配: %s", result.Name)
	}
}

func TestClient_GetDocumentChunks(t *testing.T) {
	expectedChunks := []ragflow.Chunk{
		{ID: "chunk-1", Content: "内容片段1", Similarity: 0.9},
		{ID: "chunk-2", Content: "内容片段2", Similarity: 0.8},
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET 方法, 实际 = %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedChunks))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.GetDocumentChunks(ctx, "ds-1", "doc-1", &ragflow.ListOptions{Page: 1, PageSize: 10})

	if err != nil {
		t.Fatalf("GetDocumentChunks 失败: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("期望 2 个块, 实际 = %d", len(result))
	}
}

// ==================== 聊天助手测试 ====================

func TestClient_CreateChatAssistant(t *testing.T) {
	expectedAssistant := &ragflow.ChatAssistant{
		ID:         "assistant-1",
		Name:       "测试助手",
		DatasetIDs: []string{"ds-1", "ds-2"},
	}

	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 方法, 实际 = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/chats" {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, expectedAssistant))
	})
	defer server.Close()

	ctx := context.Background()
	result, err := client.CreateChatAssistant(ctx, &ragflow.CreateChatAssistantRequest{
		Name:       "测试助手",
		DatasetIDs: []string{"ds-1", "ds-2"},
		LLMID:      "qwen-plus@Tongyi-Qianwen",
	})

	if err != nil {
		t.Fatalf("CreateChatAssistant 失败: %v", err)
	}
	if result.ID != expectedAssistant.ID {
		t.Errorf("助手 ID 不匹配")
	}
	if len(result.DatasetIDs) != 2 {
		t.Errorf("数据集 ID 数量不正确")
	}
}

func TestClient_DeleteChatAssistant(t *testing.T) {
	server, client := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("期望 DELETE 方法, 实际 = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/chats" {
			t.Errorf("路径不正确: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, nil))
	})
	defer server.Close()

	ctx := context.Background()
	err := client.DeleteChatAssistant(ctx, "assistant-1")

	if err != nil {
		t.Fatalf("DeleteChatAssistant 失败: %v", err)
	}
}
