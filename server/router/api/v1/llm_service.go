package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/usememos/memos/plugin/dashscope"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

// ==================== LLM Service 实现 ====================

// ListAvailableModels 获取可用的 LLM 模型列表
// 从 DashScope API 动态获取，带缓存（5 分钟 TTL）
func (s *APIV1Service) ListAvailableModels(ctx context.Context, _ *v1pb.ListAvailableModelsRequest) (*v1pb.ListAvailableModelsResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 检查 DashScope 客户端是否配置
	if s.DashScopeClient == nil {
		slog.Warn("ListAvailableModels: DashScope 客户端未配置，返回空列表")
		return &v1pb.ListAvailableModelsResponse{Models: []*v1pb.LLMModel{}}, nil
	}

	// 从 DashScope 获取聊天模型列表
	models, err := s.DashScopeClient.ListChatModels(ctx)
	if err != nil {
		slog.Error("ListAvailableModels: 获取模型列表失败",
			slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "failed to list models: %v", err)
	}

	// 转换为 Proto 格式
	pbModels := make([]*v1pb.LLMModel, 0, len(models))
	for _, m := range models {
		// 构造 model_id：{model_name}@Tongyi-Qianwen
		//
		// 关键修复：DashScope OpenAI 兼容模式返回的 owned_by 字段值固定为 "system"
		// （OpenAI 协议的通用占位符），这与 RAGFlow 内部的 factory 路由键
		// "Tongyi-Qianwen" 是完全不同的命名空间，不能直接使用。
		// 所有通过百炼 DashScope API 获取的模型，其 RAGFlow factory 均为 "Tongyi-Qianwen"。
		const ragflowFactory = "Tongyi-Qianwen"
		modelID := m.ModelName + "@" + ragflowFactory

		pbModels = append(pbModels, &v1pb.LLMModel{
			ModelId:     modelID,
			ModelName:   m.ModelName,
			DisplayName: m.DisplayName,
			Provider:    ragflowFactory,
			Description: m.Description,
			Status:      m.Status,
		})
	}

	slog.Info("ListAvailableModels: 返回模型列表",
		slog.Int("userID", int(userID)),
		slog.Int("modelCount", len(pbModels)))

	return &v1pb.ListAvailableModelsResponse{Models: pbModels}, nil
}

// GetUserLLMPreference 获取用户的 LLM 偏好设置
func (s *APIV1Service) GetUserLLMPreference(ctx context.Context, _ *v1pb.GetUserLLMPreferenceRequest) (*v1pb.UserLLMPreference, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 查询用户映射获取当前偏好
	mapping, err := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil {
		slog.Error("GetUserLLMPreference: 查询用户映射失败",
			slog.Int("userID", int(userID)),
			slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "failed to get user mapping: %v", err)
	}

	// 如果用户没有映射记录，返回空偏好
	if mapping == nil {
		return &v1pb.UserLLMPreference{
			ModelId:       "",
			ModelName:     "",
			Provider:      "",
			LlmConfigured: false,
		}, nil
	}

	// 解析 model_id 提取 model_name 和 provider
	modelName, provider := parseModelID(mapping.PreferredLLMID)

	return &v1pb.UserLLMPreference{
		ModelId:       mapping.PreferredLLMID,
		ModelName:     modelName,
		Provider:      provider,
		LlmConfigured: mapping.LLMConfigured,
	}, nil
}

// SetUserLLMPreference 设置用户的 LLM 偏好
// 会同步更新 RAGFlow Assistant 的 LLM 配置
func (s *APIV1Service) SetUserLLMPreference(ctx context.Context, req *v1pb.SetUserLLMPreferenceRequest) (*v1pb.UserLLMPreference, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if req.ModelId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "model_id is required")
	}

	// ==================== 向后兼容修复 ====================
	// 历史数据或旧客户端可能传入 "xxx@system"（DashScope owned_by 误用），
	// 统一重写为 "xxx@Tongyi-Qianwen"，与 RAGFlow factory 对齐。
	modelID := normalizeModelID(req.ModelId)

	slog.Info("SetUserLLMPreference: 开始设置用户 LLM 偏好",
		slog.Int("userID", int(userID)),
		slog.String("rawModelID", req.ModelId),
		slog.String("normalizedModelID", modelID))

	// 解析 model_id
	modelName, provider := parseModelID(modelID)
	if modelName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid model_id format, expected: {model_name}@{provider}")
	}

	// 验证模型是否在 RAGFlow 注册表白名单中（服务端二次校验）
	// 与 ListAvailableModels 的过滤逻辑保持一致：
	// 只有 llm_factories.json 中注册了的模型才能被 RAGFlow TenantLLM 表接受。
	// 注意：DashScope ModelExists 只验证 API Key 权限，不验证 RAGFlow 注册状态，
	// 因此此处改为白名单校验而非 DashScope 动态查询。
	if !dashscope.IsRAGFlowRegistered(modelName) {
		slog.Warn("SetUserLLMPreference: 模型未在 RAGFlow 注册表中",
			slog.String("modelName", modelName))
		return nil, status.Errorf(codes.NotFound, "model %s is not available (not registered in RAGFlow)", modelName)
	}

	// 获取用户映射
	mapping, err := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user mapping: %v", err)
	}

	// 如果用户没有映射记录，需要先通过 Provisioner 初始化
	if mapping == nil {
		if s.RAGFlowProvisioner == nil {
			return nil, status.Errorf(codes.FailedPrecondition, "RAGFlow not configured")
		}

		// 获取用户信息
		user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
		if err != nil || user == nil {
			return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
		}

		// 触发用户资源初始化
		_, _, err = s.RAGFlowProvisioner.EnsureUserResources(ctx, userID, user.Username)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to initialize user resources: %v", err)
		}

		// 重新获取映射
		mapping, err = s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
			UserID: &userID,
		})
		if err != nil || mapping == nil {
			return nil, status.Errorf(codes.Internal, "failed to get user mapping after initialization")
		}
	}

	// 确保 LLM 已配置（通过 Provisioner 自动配置百炼）
	if !mapping.LLMConfigured && s.RAGFlowProvisioner != nil {
		if err := s.RAGFlowProvisioner.EnsureLLMConfig(ctx, userID); err != nil {
			slog.Warn("SetUserLLMPreference: 自动配置 LLM 失败",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
			// 继续执行，LLM 配置失败不阻塞偏好设置
		}
	}

	// 更新 RAGFlow Assistant 的 LLM 配置（Assistant 已存在时）
	if mapping.AssistantID != "" && s.RAGFlowProvisioner != nil {
		if err := s.RAGFlowProvisioner.UpdateUserAssistantLLM(ctx, userID, modelID); err != nil {
			slog.Error("SetUserLLMPreference: 更新 Assistant LLM 失败",
				slog.Int("userID", int(userID)),
				slog.String("assistantID", mapping.AssistantID),
				slog.Any("error", err))
			return nil, status.Errorf(codes.Internal, "failed to update assistant LLM: %v", err)
		}
	}

	// 更新数据库中的用户偏好（存储已规范化的 model_id）
	now := time.Now().Unix()
	if err := s.Store.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
		ID:             mapping.ID,
		PreferredLLMID: &modelID,
		UpdatedTs:      &now,
	}); err != nil {
		slog.Error("SetUserLLMPreference: 更新用户偏好失败",
			slog.Int("userID", int(userID)),
			slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "failed to update user preference: %v", err)
	}

	slog.Info("SetUserLLMPreference: 用户 LLM 偏好设置成功",
		slog.Int("userID", int(userID)),
		slog.String("modelID", modelID))

	// ==================== 补偿创建 Assistant ====================
	if mapping.AssistantID == "" && s.RAGFlowProvisioner != nil {
		go func() {
			user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
			if err != nil || user == nil {
				slog.Warn("SetUserLLMPreference: 补偿创建 Assistant 时获取用户信息失败",
					slog.Int("userID", int(userID)))
				return
			}
			slog.Info("SetUserLLMPreference: 触发 Assistant 补偿创建",
				slog.Int("userID", int(userID)),
				slog.String("modelID", modelID))
			if _, _, err := s.RAGFlowProvisioner.EnsureUserResources(ctx, userID, user.Username); err != nil {
				slog.Warn("SetUserLLMPreference: 补偿创建 Assistant 失败",
					slog.Int("userID", int(userID)),
					slog.Any("error", err))
			}
		}()
	}

	// 重新查询映射获取最新状态
	mapping, _ = s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})

	llmConfigured := false
	if mapping != nil {
		llmConfigured = mapping.LLMConfigured
	}

	return &v1pb.UserLLMPreference{
		ModelId:       modelID,
		ModelName:     modelName,
		Provider:      provider,
		LlmConfigured: llmConfigured,
	}, nil
}

// ==================== Chat Settings Service 实现 ====================

// ListDatasets 获取用户可用的 Dataset 列表
func (s *APIV1Service) ListDatasets(ctx context.Context, _ *v1pb.ListDatasetsRequest) (*v1pb.ListDatasetsResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 获取用户的 RAGFlow 客户端
	client := s.getUserRAGFlowClient(ctx, userID)
	if client == nil {
		slog.Warn("ListDatasets: 用户未配置 RAGFlow，返回空列表",
			slog.Int("userID", int(userID)))
		return &v1pb.ListDatasetsResponse{Datasets: []*v1pb.Dataset{}}, nil
	}

	// 获取用户映射查看默认 Dataset
	mapping, _ := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})

	defaultDatasetID := ""
	if mapping != nil {
		defaultDatasetID = mapping.DatasetID
	}

	// 从 RAGFlow 获取 Dataset 列表
	datasets, err := client.ListDatasets(ctx, nil)
	if err != nil {
		slog.Error("ListDatasets: 获取 Dataset 列表失败",
			slog.Int("userID", int(userID)),
			slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "failed to list datasets: %v", err)
	}

	// 转换为 Proto 格式
	pbDatasets := make([]*v1pb.Dataset, 0, len(datasets))
	for _, ds := range datasets {
		pbDatasets = append(pbDatasets, &v1pb.Dataset{
			Id:            ds.ID,
			Name:          ds.Name,
			Description:   ds.Description,
			DocumentCount: int32(ds.DocumentCount),
			IsDefault:     ds.ID == defaultDatasetID,
		})
	}

	return &v1pb.ListDatasetsResponse{Datasets: pbDatasets}, nil
}

// GetChatSettings 获取用户的聊天设置
func (s *APIV1Service) GetChatSettings(ctx context.Context, _ *v1pb.GetChatSettingsRequest) (*v1pb.ChatSettings, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 查询用户映射
	mapping, err := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user mapping: %v", err)
	}

	// 默认设置
	settings := &v1pb.ChatSettings{
		DatasetIds:       []string{},
		QuoteEnabled:     true,  // 默认开启 RAG 引用
		ReasoningEnabled: false, // 默认关闭深度研究
	}

	if mapping != nil {
		// 解析 dataset_ids JSON
		if mapping.DatasetIDs != "" && mapping.DatasetIDs != "[]" {
			var datasetIDs []string
			if err := json.Unmarshal([]byte(mapping.DatasetIDs), &datasetIDs); err == nil {
				settings.DatasetIds = datasetIDs
			}
		}

		// 如果没有选择 Dataset，使用默认 Dataset
		if len(settings.DatasetIds) == 0 && mapping.DatasetID != "" {
			settings.DatasetIds = []string{mapping.DatasetID}
		}

		// 从 mapping 读取 quote 和 reasoning 设置
		settings.QuoteEnabled = mapping.QuoteEnabled
		settings.ReasoningEnabled = mapping.ReasoningEnabled
	}

	// 获取可用的 Dataset 列表
	datasetsResp, err := s.ListDatasets(ctx, &v1pb.ListDatasetsRequest{})
	if err == nil && datasetsResp != nil {
		settings.AvailableDatasets = datasetsResp.Datasets
	}

	return settings, nil
}

// UpdateChatSettings 更新用户的聊天设置
func (s *APIV1Service) UpdateChatSettings(ctx context.Context, req *v1pb.UpdateChatSettingsRequest) (*v1pb.ChatSettings, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	slog.Info("UpdateChatSettings: 开始更新聊天设置",
		slog.Int("userID", int(userID)),
		slog.Any("datasetIDs", req.DatasetIds),
		slog.Any("quoteEnabled", req.QuoteEnabled),
		slog.Any("reasoningEnabled", req.ReasoningEnabled))

	// 获取用户映射
	mapping, err := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user mapping: %v", err)
	}

	if mapping == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "user not initialized for RAGFlow")
	}

	// 准备更新
	update := &store.UpdateRAGFlowUserMapping{
		ID: mapping.ID,
	}
	now := time.Now().Unix()
	update.UpdatedTs = &now

	// 更新 Dataset 选择
	if len(req.DatasetIds) > 0 {
		// 验证 Dataset ID 存在（可选，这里简化处理）
		datasetIDsJSON, err := json.Marshal(req.DatasetIds)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to serialize dataset_ids: %v", err)
		}
		datasetIDsStr := string(datasetIDsJSON)
		update.DatasetIDs = &datasetIDsStr

		// 同步更新 RAGFlow Assistant 的 Dataset 配置
		if mapping.AssistantID != "" && s.RAGFlowProvisioner != nil {
			if err := s.RAGFlowProvisioner.UpdateUserAssistantDatasets(ctx, userID, req.DatasetIds); err != nil {
				slog.Error("UpdateChatSettings: 更新 Assistant Dataset 失败",
					slog.Int("userID", int(userID)),
					slog.Any("error", err))
				return nil, status.Errorf(codes.Internal, "failed to update assistant datasets: %v", err)
			}
		}
	}

	// 更新 Quote 开关
	if req.QuoteEnabled != nil {
		update.QuoteEnabled = req.QuoteEnabled
	}

	// 更新 Reasoning 开关
	if req.ReasoningEnabled != nil {
		update.ReasoningEnabled = req.ReasoningEnabled
	}

	// 执行更新
	if err := s.Store.UpdateRAGFlowUserMapping(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update chat settings: %v", err)
	}

	slog.Info("UpdateChatSettings: 聊天设置更新成功",
		slog.Int("userID", int(userID)))

	// 返回更新后的设置
	return s.GetChatSettings(ctx, &v1pb.GetChatSettingsRequest{})
}

// ==================== 辅助函数 ====================

// normalizeModelID 将 model_id 中不合法的 provider 修正为 RAGFlow factory 名称
//
// 背景：DashScope OpenAI 兼容 API 的 owned_by 字段恒为 "system"，
// 这是 OpenAI 协议的通用占位符，与 RAGFlow factory "Tongyi-Qianwen" 不同。
// 此函数将 "@system" 统一重写为 "@Tongyi-Qianwen"，确保下游 RAGFlow 调用正确路由。
func normalizeModelID(modelID string) string {
	if strings.HasSuffix(modelID, "@system") {
		modelName := strings.TrimSuffix(modelID, "@system")
		return modelName + "@Tongyi-Qianwen"
	}
	return modelID
}

// parseModelID 解析 model_id 格式：{model_name}@{provider}
// 返回 model_name 和 provider，如果格式无效返回空字符串
func parseModelID(modelID string) (modelName, provider string) {
	if modelID == "" {
		return "", ""
	}

	parts := strings.SplitN(modelID, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	// 如果没有 @ 分隔符，整体作为 model_name
	return modelID, ""
}
