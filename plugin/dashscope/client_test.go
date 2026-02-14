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

// mockModelsResponse 创建模拟的模型列表响应
func mockModelsResponse(models []Model, pageNo, pageSize, total int) ModelsResponse {
	totalPages := (total + pageSize - 1) / pageSize
	return ModelsResponse{
		Output: struct {
			Data       []Model `json:"data"`
			Total      int     `json:"total"`
			PageNo     int     `json:"page_no"`
			PageSize   int     `json:"page_size"`
			TotalPages int     `json:"total_pages"`
		}{
			Data:       models,
			Total:      total,
			PageNo:     pageNo,
			PageSize:   pageSize,
			TotalPages: totalPages,
		},
		RequestID: "test-request-id",
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
	// 创建模拟服务器
	models := []Model{
		{
			ModelID:     "model-1",
			ModelName:   "qwen-plus",
			DisplayName: "通义千问-Plus",
			ModelType:   "text-generation",
			Status:      "RUNNING",
		},
		{
			ModelID:     "model-2",
			ModelName:   "qwen-max",
			DisplayName: "通义千问-Max",
			ModelType:   "text-generation",
			Status:      "RUNNING",
		},
		{
			ModelID:     "model-3",
			ModelName:   "text-embedding-v1",
			DisplayName: "文本嵌入模型",
			ModelType:   "embeddings",
			Status:      "RUNNING",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/api/v1/deployments/models")

		resp := mockModelsResponse(models, 1, 100, len(models))
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
	models := []Model{
		{ModelName: "qwen-plus", ModelType: "text-generation", Status: "RUNNING"},
		{ModelName: "qwen-max", ModelType: "text-generation", Status: "RUNNING"},
		{ModelName: "text-embedding-v1", ModelType: "embeddings", Status: "RUNNING"},
		{ModelName: "stopped-model", ModelType: "text-generation", Status: "STOPPED"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockModelsResponse(models, 1, 100, len(models))
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
	assert.Len(t, result, 2) // 只返回 RUNNING 的 text-generation 模型
	assert.Equal(t, "qwen-plus", result[0].ModelName)
	assert.Equal(t, "qwen-max", result[1].ModelName)
}

func TestClient_Cache(t *testing.T) {
	callCount := 0
	models := []Model{
		{ModelName: "qwen-plus", ModelType: "text-generation", Status: "RUNNING"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := mockModelsResponse(models, 1, 100, len(models))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		APIKey:   "sk-test",
		BaseURL:  server.URL,
		CacheTTL: 1 * time.Hour, // 长缓存时间
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
	models := []Model{
		{ModelName: "qwen-plus", ModelType: "text-generation", Status: "RUNNING"},
		{ModelName: "qwen-max", ModelType: "text-generation", Status: "RUNNING"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockModelsResponse(models, 1, 100, len(models))
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

	// 存在的模型
	model, err := client.GetModel(ctx, "qwen-plus")
	assert.NoError(t, err)
	assert.Equal(t, "qwen-plus", model.ModelName)

	// 不存在的模型
	_, err = client.GetModel(ctx, "non-existent")
	assert.Error(t, err)
}

func TestClient_ModelExists(t *testing.T) {
	models := []Model{
		{ModelName: "qwen-plus", ModelType: "text-generation", Status: "RUNNING"},
		{ModelName: "stopped-model", ModelType: "text-generation", Status: "STOPPED"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockModelsResponse(models, 1, 100, len(models))
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

	// 存在且运行中
	assert.True(t, client.ModelExists(ctx, "qwen-plus"))

	// 存在但已停止
	assert.False(t, client.ModelExists(ctx, "stopped-model"))

	// 不存在
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
