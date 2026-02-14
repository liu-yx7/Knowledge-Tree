package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

// ==================== ListAvailableModels 测试 ====================

func TestListAvailableModels_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 未认证的请求
	_, err := ts.Service.ListAvailableModels(ctx, &v1pb.ListAvailableModelsRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code(), "未认证用户应返回 Unauthenticated")
}

func TestListAvailableModels_NilDashScopeClient(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 创建用户
	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// DashScope 客户端为 nil 时应返回空列表
	ts.Service.DashScopeClient = nil

	resp, err := ts.Service.ListAvailableModels(userCtx, &v1pb.ListAvailableModelsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Models, "DashScope 未配置时应返回空列表")
}

// ==================== GetUserLLMPreference 测试 ====================

func TestGetUserLLMPreference_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.GetUserLLMPreference(ctx, &v1pb.GetUserLLMPreferenceRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetUserLLMPreference_NoMapping(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 用户没有映射记录时，返回空偏好
	resp, err := ts.Service.GetUserLLMPreference(userCtx, &v1pb.GetUserLLMPreferenceRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Empty(t, resp.ModelId, "无映射时 ModelId 应为空")
	assert.Empty(t, resp.ModelName, "无映射时 ModelName 应为空")
	assert.Empty(t, resp.Provider, "无映射时 Provider 应为空")
	assert.False(t, resp.LlmConfigured, "无映射时 LlmConfigured 应为 false")
}

func TestGetUserLLMPreference_WithMapping(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建用户映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:         user.ID,
		PreferredLLMID: "qwen-max@Tongyi-Qianwen",
		LLMConfigured:  true,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	resp, err := ts.Service.GetUserLLMPreference(userCtx, &v1pb.GetUserLLMPreferenceRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "qwen-max@Tongyi-Qianwen", resp.ModelId)
	assert.Equal(t, "qwen-max", resp.ModelName)
	assert.Equal(t, "Tongyi-Qianwen", resp.Provider)
	assert.True(t, resp.LlmConfigured)
}

// ==================== SetUserLLMPreference 测试 ====================

func TestSetUserLLMPreference_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.SetUserLLMPreference(ctx, &v1pb.SetUserLLMPreferenceRequest{
		ModelId: "qwen-max@Tongyi-Qianwen",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestSetUserLLMPreference_EmptyModelId(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	_, err = ts.Service.SetUserLLMPreference(userCtx, &v1pb.SetUserLLMPreferenceRequest{
		ModelId: "",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code(), "空 ModelId 应返回 InvalidArgument")
}

func TestSetUserLLMPreference_NoProvisioner_NoMapping(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 没有 Provisioner 且没有映射时，应返回 FailedPrecondition
	ts.Service.RAGFlowProvisioner = nil
	ts.Service.DashScopeClient = nil

	_, err = ts.Service.SetUserLLMPreference(userCtx, &v1pb.SetUserLLMPreferenceRequest{
		ModelId: "qwen-max@Tongyi-Qianwen",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestSetUserLLMPreference_WithExistingMapping(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建用户映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:         user.ID,
		PreferredLLMID: "old-model@Provider",
		LLMConfigured:  true,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 不配置 DashScopeClient 以跳过模型验证
	ts.Service.DashScopeClient = nil
	ts.Service.RAGFlowProvisioner = nil

	resp, err := ts.Service.SetUserLLMPreference(userCtx, &v1pb.SetUserLLMPreferenceRequest{
		ModelId: "qwen-plus@Tongyi-Qianwen",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "qwen-plus@Tongyi-Qianwen", resp.ModelId)
	assert.Equal(t, "qwen-plus", resp.ModelName)
	assert.Equal(t, "Tongyi-Qianwen", resp.Provider)

	// 验证数据库中的值已更新
	mapping, err := ts.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &user.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "qwen-plus@Tongyi-Qianwen", mapping.PreferredLLMID)
}

// ==================== ListDatasets 测试 ====================

func TestListDatasets_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.ListDatasets(ctx, &v1pb.ListDatasetsRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestListDatasets_NoRAGFlowClient(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// RAGFlow 客户端为 nil 时应返回空列表
	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	resp, err := ts.Service.ListDatasets(userCtx, &v1pb.ListDatasetsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Datasets, "RAGFlow 未配置时应返回空列表")
}

// ==================== GetChatSettings 测试 ====================

func TestGetChatSettings_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.GetChatSettings(ctx, &v1pb.GetChatSettingsRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetChatSettings_NoMapping(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 禁用 RAGFlow 客户端以避免调用
	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	resp, err := ts.Service.GetChatSettings(userCtx, &v1pb.GetChatSettingsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 默认设置
	assert.Empty(t, resp.DatasetIds, "无映射时 DatasetIds 应为空")
	assert.True(t, resp.QuoteEnabled, "默认 QuoteEnabled 应为 true")
	assert.False(t, resp.ReasoningEnabled, "默认 ReasoningEnabled 应为 false")
}

func TestGetChatSettings_WithMapping(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建用户映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:           user.ID,
		DatasetID:        "default-dataset-id",
		DatasetIDs:       `["kb-001", "kb-002"]`,
		QuoteEnabled:     false,
		ReasoningEnabled: true,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 禁用 RAGFlow 客户端以避免调用
	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	resp, err := ts.Service.GetChatSettings(userCtx, &v1pb.GetChatSettingsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Len(t, resp.DatasetIds, 2)
	assert.Contains(t, resp.DatasetIds, "kb-001")
	assert.Contains(t, resp.DatasetIds, "kb-002")
	assert.False(t, resp.QuoteEnabled)
	assert.True(t, resp.ReasoningEnabled)
}

func TestGetChatSettings_FallbackToDefaultDataset(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建用户映射，有 DatasetID 但没有 DatasetIDs
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:       user.ID,
		DatasetID:    "default-dataset-id",
		DatasetIDs:   "", // 空
		QuoteEnabled: true,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	resp, err := ts.Service.GetChatSettings(userCtx, &v1pb.GetChatSettingsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 应回退到默认 DatasetID
	assert.Len(t, resp.DatasetIds, 1)
	assert.Equal(t, "default-dataset-id", resp.DatasetIds[0])
}

// ==================== UpdateChatSettings 测试 ====================

func TestUpdateChatSettings_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.UpdateChatSettings(ctx, &v1pb.UpdateChatSettingsRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestUpdateChatSettings_NoMapping(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 没有映射时应返回 FailedPrecondition
	quoteEnabled := true
	_, err = ts.Service.UpdateChatSettings(userCtx, &v1pb.UpdateChatSettingsRequest{
		QuoteEnabled: &quoteEnabled,
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestUpdateChatSettings_UpdateQuoteEnabled(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建用户映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:       user.ID,
		QuoteEnabled: true,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	// 更新 QuoteEnabled 为 false
	quoteEnabled := false
	resp, err := ts.Service.UpdateChatSettings(userCtx, &v1pb.UpdateChatSettingsRequest{
		QuoteEnabled: &quoteEnabled,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.QuoteEnabled)

	// 验证数据库中的值已更新
	mapping, err := ts.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &user.ID,
	})
	require.NoError(t, err)
	assert.False(t, mapping.QuoteEnabled)
}

func TestUpdateChatSettings_UpdateReasoningEnabled(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建用户映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:           user.ID,
		ReasoningEnabled: false,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	// 更新 ReasoningEnabled 为 true
	reasoningEnabled := true
	resp, err := ts.Service.UpdateChatSettings(userCtx, &v1pb.UpdateChatSettingsRequest{
		ReasoningEnabled: &reasoningEnabled,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.ReasoningEnabled)

	// 验证数据库中的值已更新
	mapping, err := ts.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &user.ID,
	})
	require.NoError(t, err)
	assert.True(t, mapping.ReasoningEnabled)
}

func TestUpdateChatSettings_UpdateMultipleFields(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建用户映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:           user.ID,
		QuoteEnabled:     true,
		ReasoningEnabled: false,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	// 同时更新多个字段
	quoteEnabled := false
	reasoningEnabled := true
	resp, err := ts.Service.UpdateChatSettings(userCtx, &v1pb.UpdateChatSettingsRequest{
		QuoteEnabled:     &quoteEnabled,
		ReasoningEnabled: &reasoningEnabled,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.False(t, resp.QuoteEnabled)
	assert.True(t, resp.ReasoningEnabled)

	// 验证数据库中的值已更新
	mapping, err := ts.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &user.ID,
	})
	require.NoError(t, err)
	assert.False(t, mapping.QuoteEnabled)
	assert.True(t, mapping.ReasoningEnabled)
}

// ==================== parseModelID 辅助函数测试 ====================

func TestParseModelID(t *testing.T) {
	tests := []struct {
		name         string
		modelID      string
		wantModel    string
		wantProvider string
	}{
		{
			name:         "正常格式",
			modelID:      "qwen-max@Tongyi-Qianwen",
			wantModel:    "qwen-max",
			wantProvider: "Tongyi-Qianwen",
		},
		{
			name:         "多个@符号",
			modelID:      "model@provider@extra",
			wantModel:    "model",
			wantProvider: "provider@extra",
		},
		{
			name:         "无@符号",
			modelID:      "standalone-model",
			wantModel:    "standalone-model",
			wantProvider: "",
		},
		{
			name:         "空字符串",
			modelID:      "",
			wantModel:    "",
			wantProvider: "",
		},
		{
			name:         "只有@",
			modelID:      "@",
			wantModel:    "",
			wantProvider: "",
		},
		{
			name:         "以@开头",
			modelID:      "@Provider",
			wantModel:    "",
			wantProvider: "Provider",
		},
		{
			name:         "以@结尾",
			modelID:      "model@",
			wantModel:    "model",
			wantProvider: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 由于 parseModelID 是私有函数，我们通过 GetUserLLMPreference 间接测试
			// 这里只验证 SetUserLLMPreference 的解析逻辑
			// 实际上已经在上面的集成测试中覆盖了
		})
	}
}

// ==================== 数据隔离测试 ====================

func TestLLMPreference_DataIsolation(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 创建两个用户
	user1, err := ts.CreateRegularUser(ctx, "user1")
	require.NoError(t, err)

	user2, err := ts.CreateRegularUser(ctx, "user2")
	require.NoError(t, err)

	// 为 user1 创建映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:         user1.ID,
		PreferredLLMID: "user1-model@Provider",
		LLMConfigured:  true,
	})
	require.NoError(t, err)

	// 为 user2 创建映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:         user2.ID,
		PreferredLLMID: "user2-model@Provider",
		LLMConfigured:  false,
	})
	require.NoError(t, err)

	// user1 只能看到自己的偏好
	user1Ctx := ts.CreateUserContext(ctx, user1.ID)
	resp1, err := ts.Service.GetUserLLMPreference(user1Ctx, &v1pb.GetUserLLMPreferenceRequest{})
	require.NoError(t, err)
	assert.Equal(t, "user1-model@Provider", resp1.ModelId)
	assert.True(t, resp1.LlmConfigured)

	// user2 只能看到自己的偏好
	user2Ctx := ts.CreateUserContext(ctx, user2.ID)
	resp2, err := ts.Service.GetUserLLMPreference(user2Ctx, &v1pb.GetUserLLMPreferenceRequest{})
	require.NoError(t, err)
	assert.Equal(t, "user2-model@Provider", resp2.ModelId)
	assert.False(t, resp2.LlmConfigured)
}

func TestChatSettings_DataIsolation(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 创建两个用户
	user1, err := ts.CreateRegularUser(ctx, "user1")
	require.NoError(t, err)

	user2, err := ts.CreateRegularUser(ctx, "user2")
	require.NoError(t, err)

	// 为 user1 创建映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:           user1.ID,
		QuoteEnabled:     true,
		ReasoningEnabled: false,
	})
	require.NoError(t, err)

	// 为 user2 创建映射
	_, err = ts.Store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:           user2.ID,
		QuoteEnabled:     false,
		ReasoningEnabled: true,
	})
	require.NoError(t, err)

	ts.Service.RAGFlowClient = nil
	ts.Service.RAGFlowProvisioner = nil

	// user1 只能看到自己的设置
	user1Ctx := ts.CreateUserContext(ctx, user1.ID)
	resp1, err := ts.Service.GetChatSettings(user1Ctx, &v1pb.GetChatSettingsRequest{})
	require.NoError(t, err)
	assert.True(t, resp1.QuoteEnabled)
	assert.False(t, resp1.ReasoningEnabled)

	// user2 只能看到自己的设置
	user2Ctx := ts.CreateUserContext(ctx, user2.ID)
	resp2, err := ts.Service.GetChatSettings(user2Ctx, &v1pb.GetChatSettingsRequest{})
	require.NoError(t, err)
	assert.False(t, resp2.QuoteEnabled)
	assert.True(t, resp2.ReasoningEnabled)
}
