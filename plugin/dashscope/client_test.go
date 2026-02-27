package dashscope

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== 测试辅助 ====================

// mockOpenAIModelsResponse 创建模拟的 OpenAI 兼容模式模型列表响应
func mockOpenAIModelsResponse(models []OpenAIModel) OpenAIModelsResponse {
	return OpenAIModelsResponse{
		Object: "list",
		Data:   models,
	}
}

// ==================== 配置测试 ====================

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "空 API Key",
			config:  &Config{APIKey: ""},
			wantErr: true,
		},
		{
			name:    "有效配置",
			config:  &Config{APIKey: "sk-test"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_WithDefaults(t *testing.T) {
	cfg := &Config{APIKey: "sk-test"}
	cfg = cfg.WithDefaults()

	assert.Equal(t, "https://dashscope.aliyuncs.com", cfg.BaseURL)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 5*time.Minute, cfg.CacheTTL)
}

// ==================== 客户端测试 ====================

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "空配置使用默认值但缺少 APIKey",
			config:  nil,
			wantErr: true,
		},
		{
			name:    "缺少 APIKey",
			config:  &Config{},
			wantErr: true,
		},
		{
			name:    "有效配置",
			config:  &Config{APIKey: "sk-test"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestClient_ListModels(t *testing.T) {
	openAIModels := []OpenAIModel{
		{ID: "qwen-plus", Object: "model", Created: 1000, OwnedBy: "system"},
		{ID: "qwen-max", Object: "model", Created: 1000, OwnedBy: "system"},
		{ID: "text-embedding-v2", Object: "model", Created: 1000, OwnedBy: "system"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/compatible-mode/v1/models")

		resp := mockOpenAIModelsResponse(openAIModels)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.ListModels(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "qwen-plus", result[0].ModelName)
}

func TestClient_ListChatModels(t *testing.T) {
	// qwen-plus 和 qwen-max 在白名单中，deepseek-r1 也在，
	// text-embedding-v2 被 isChatModel 过滤，
	// qwen3.5-plus 被 isRAGFlowRegistered 过滤（不在 factories.json）
	openAIModels := []OpenAIModel{
		{ID: "qwen-plus", Object: "model", Created: 1000, OwnedBy: "system"},
		{ID: "qwen-max", Object: "model", Created: 1000, OwnedBy: "system"},
		{ID: "deepseek-r1", Object: "model", Created: 1000, OwnedBy: "system"},
		{ID: "text-embedding-v2", Object: "model", Created: 1000, OwnedBy: "system"},
		{ID: "qwen3.5-plus", Object: "model", Created: 1000, OwnedBy: "system"}, // 不在 RAGFlow 注册表
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockOpenAIModelsResponse(openAIModels)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.ListChatModels(ctx)

	assert.NoError(t, err)
	// 只返回：qwen-plus、qwen-max、deepseek-r1（均在白名单中）
	// 排除：text-embedding-v2（非聊天）、qwen3.5-plus（不在 RAGFlow 注册表）
	assert.Len(t, result, 3)

	names := make([]string, len(result))
	for i, m := range result {
		names[i] = m.ModelName
	}
	assert.Contains(t, names, "qwen-plus")
	assert.Contains(t, names, "qwen-max")
	assert.Contains(t, names, "deepseek-r1")
	assert.NotContains(t, names, "qwen3.5-plus")
	assert.NotContains(t, names, "text-embedding-v2")
}

func TestClient_Cache(t *testing.T) {
	callCount := 0
	openAIModels := []OpenAIModel{
		{ID: "qwen-plus", Object: "model", Created: 1000, OwnedBy: "system"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := mockOpenAIModelsResponse(openAIModels)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		APIKey:   "sk-test",
		BaseURL:  server.URL,
		CacheTTL: 1 * time.Hour,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 第一次调用应该请求 API
	_, err = client.ListModels(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// 第二次调用应该使用缓存
	_, err = client.ListModels(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount) // 计数不变

	// 强制刷新后应该再次请求 API
	err = client.RefreshCache(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestClient_GetModel(t *testing.T) {
	openAIModels := []OpenAIModel{
		{ID: "qwen-plus", Object: "model", Created: 1000, OwnedBy: "system"},
		{ID: "qwen-max", Object: "model", Created: 1000, OwnedBy: "system"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockOpenAIModelsResponse(openAIModels)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	ctx := context.Background()

	model, err := client.GetModel(ctx, "qwen-plus")
	assert.NoError(t, err)
	assert.Equal(t, "qwen-plus", model.ModelName)

	_, err = client.GetModel(ctx, "non-existent")
	assert.Error(t, err)
}

func TestClient_ModelExists(t *testing.T) {
	openAIModels := []OpenAIModel{
		{ID: "qwen-plus", Object: "model", Created: 1000, OwnedBy: "system"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockOpenAIModelsResponse(openAIModels)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	ctx := context.Background()

	assert.True(t, client.ModelExists(ctx, "qwen-plus"))
	assert.False(t, client.ModelExists(ctx, "non-existent"))
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.ListModels(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// ==================== 白名单测试 ====================

func TestIsRAGFlowRegistered(t *testing.T) {
	// 白名单内的模型
	assert.True(t, IsRAGFlowRegistered("qwen-plus"))
	assert.True(t, IsRAGFlowRegistered("qwen-max"))
	assert.True(t, IsRAGFlowRegistered("deepseek-r1"))
	assert.True(t, IsRAGFlowRegistered("qwq-plus"))
	assert.True(t, IsRAGFlowRegistered("Moonshot-Kimi-K2-Instruct"))
	assert.True(t, IsRAGFlowRegistered("qwen3-32b"))

	// 白名单外的模型（DashScope 有但 RAGFlow 未注册）
	assert.False(t, IsRAGFlowRegistered("qwen3.5-plus"))
	assert.False(t, IsRAGFlowRegistered("qwen-turbo-0919"))
	assert.False(t, IsRAGFlowRegistered("some-future-model"))
	assert.False(t, IsRAGFlowRegistered(""))
}
