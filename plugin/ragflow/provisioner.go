package ragflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/usememos/memos/store"
)

// ==================== Provisioner 定义 ====================

// Provisioner RAGFlow 用户自动配置器
// 职责：为 Memos 用户透明地配置 RAGFlow 的全部资源：
//  1. 认证配置：注册/登录/生成 API Key
//  2. 资源配置：创建 Dataset、Chat Assistant
//
// 这是用户 RAGFlow 资源的唯一编排入口，Orchestrator 和 Service 层
// 都通过 Provisioner 获取 per-user Client 和资源 ID。
//
// 使用方式：
//
//	provisioner, _ := NewProvisioner(cfg, store)
//	client, _ := provisioner.GetClientForUser(ctx, memosUserID, username)
//	// client 可直接调用 RAGFlow API（CreateDataset, Retrieve, Chat 等）
type Provisioner struct {
	config     *Config
	authClient *AuthClient
	store      ProvisionerStore

	// clientCache 缓存已创建的 Client 实例，避免重复创建
	clientCache map[int32]*Client
	mu          sync.RWMutex
}

// ProvisionerStore 定义 Provisioner 所需的存储接口
// 使用接口而非具体 Store 类型，便于测试时注入 Mock 实现
type ProvisionerStore interface {
	GetRAGFlowUserMapping(ctx context.Context, find *store.FindRAGFlowUserMapping) (*store.RAGFlowUserMapping, error)
	CreateRAGFlowUserMapping(ctx context.Context, create *store.RAGFlowUserMapping) (*store.RAGFlowUserMapping, error)
	UpdateRAGFlowUserMapping(ctx context.Context, update *store.UpdateRAGFlowUserMapping) error
}

// ==================== 构造函数 ====================

// NewProvisioner 创建 RAGFlow 用户自动配置器
// 参数:
//   - cfg: RAGFlow 配置（需包含 BaseURL 和公钥配置）
//   - s: 数据库存储接口
//
// 返回: Provisioner 实例或错误
func NewProvisioner(cfg *Config, s ProvisionerStore) (*Provisioner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("配置不能为空")
	}
	if s == nil {
		return nil, fmt.Errorf("存储接口不能为空")
	}

	authClient, err := NewAuthClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建认证客户端失败: %w", err)
	}

	return &Provisioner{
		config:      cfg,
		authClient:  authClient,
		store:       s,
		clientCache: make(map[int32]*Client),
	}, nil
}

// NewProvisionerWithAuthClient 创建 Provisioner（注入 AuthClient，用于测试）
func NewProvisionerWithAuthClient(cfg *Config, s ProvisionerStore, authClient *AuthClient) (*Provisioner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("配置不能为空")
	}
	if s == nil {
		return nil, fmt.Errorf("存储接口不能为空")
	}
	if authClient == nil {
		return nil, fmt.Errorf("认证客户端不能为空")
	}

	return &Provisioner{
		config:      cfg,
		authClient:  authClient,
		store:       s,
		clientCache: make(map[int32]*Client),
	}, nil
}

// ==================== 核心方法 ====================

// GetClientForUser 获取用户的 RAGFlow 客户端
// 如果用户已有映射且 API Key 可用，直接返回缓存/新建的 Client；
// 如果用户未配置或 API Key 缺失，自动触发配置流程。
//
// 这是 Provisioner 的主入口，Memos 服务层应通过此方法获取 RAGFlow 客户端。
func (p *Provisioner) GetClientForUser(ctx context.Context, memosUserID int32, username string) (*Client, error) {
	// 快速路径：从缓存中获取
	p.mu.RLock()
	if client, ok := p.clientCache[memosUserID]; ok {
		p.mu.RUnlock()
		return client, nil
	}
	p.mu.RUnlock()

	// 慢路径：查询数据库或执行配置
	client, err := p.provisionUser(ctx, memosUserID, username)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	p.mu.Lock()
	p.clientCache[memosUserID] = client
	p.mu.Unlock()

	return client, nil
}

// EnsureUserResources 确保用户的全部 RAGFlow 资源就绪（认证 + Dataset + Assistant）
// 返回 per-user Client 和 DatasetID
// 这是面向 Orchestrator 的入口，同步事件处理时调用此方法
func (p *Provisioner) EnsureUserResources(ctx context.Context, memosUserID int32, username string) (*Client, string, error) {
	// 1. 确保认证配置（API Key）
	client, err := p.GetClientForUser(ctx, memosUserID, username)
	if err != nil {
		return nil, "", fmt.Errorf("确保用户认证配置失败: %w", err)
	}

	// 2. 确保 Dataset 存在
	datasetID, err := p.ensureDataset(ctx, client, memosUserID)
	if err != nil {
		return nil, "", fmt.Errorf("确保用户 Dataset 失败: %w", err)
	}

	// 3. 确保 Assistant 存在（独立检查，覆盖 ensureDataset 走快速路径的情况）
	// 当 DatasetID 已存在但 AssistantID 为空（半初始化状态），ensureDataset 不会触发
	// ensureAssistant，需在此处独立补偿。
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err == nil && mapping != nil && mapping.AssistantID == "" {
		p.ensureAssistant(ctx, client, memosUserID, mapping.ID, datasetID)
	}

	return client, datasetID, nil
}

// GetUserDatasetID 获取用户的 Dataset ID，不存在则自动创建
// 这是面向 SemanticSearch 等查询场景的入口
func (p *Provisioner) GetUserDatasetID(ctx context.Context, memosUserID int32, username string) (string, error) {
	// 先查映射表，快速路径
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return "", fmt.Errorf("查询用户映射失败: %w", err)
	}

	// 如果 DatasetID 已存在，直接返回
	if mapping != nil && mapping.DatasetID != "" {
		return mapping.DatasetID, nil
	}

	// DatasetID 不存在，触发完整资源配置
	_, datasetID, err := p.EnsureUserResources(ctx, memosUserID, username)
	if err != nil {
		return "", err
	}

	return datasetID, nil
}

// InvalidateCache 清除指定用户的缓存（用于 API Key 失效后重新配置）
func (p *Provisioner) InvalidateCache(memosUserID int32) {
	p.mu.Lock()
	delete(p.clientCache, memosUserID)
	p.mu.Unlock()
}

// ==================== 资源配置：Dataset 和 Assistant ====================

// ensureDataset 确保用户有对应的 RAGFlow Dataset
// 如果映射中已有 DatasetID，直接返回；否则创建新 Dataset
func (p *Provisioner) ensureDataset(ctx context.Context, client *Client, memosUserID int32) (string, error) {
	// 查询当前映射
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return "", fmt.Errorf("查询用户映射失败: %w", err)
	}
	if mapping == nil {
		return "", fmt.Errorf("用户映射不存在，需先完成认证配置")
	}

	// 已有 DatasetID，直接返回
	if mapping.DatasetID != "" {
		return mapping.DatasetID, nil
	}

	// 使用 per-user Client 创建 Dataset
	datasetName := fmt.Sprintf("knowtree_user_%d", memosUserID)
	dataset, err := client.CreateDataset(ctx, &CreateDatasetRequest{
		Name:        datasetName,
		Description: fmt.Sprintf("Knowtree 用户 %d 的知识库", memosUserID),
		ChunkMethod: ChunkMethodNaive,
	})
	if err != nil {
		return "", fmt.Errorf("创建 RAGFlow Dataset 失败: %w", err)
	}

	slog.Info("为用户创建了新的 RAGFlow Dataset",
		slog.Int("userID", int(memosUserID)),
		slog.String("datasetID", dataset.ID),
		slog.String("datasetName", datasetName))

	// 更新映射
	now := time.Now().Unix()
	if err := p.store.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
		ID:          mapping.ID,
		DatasetID:   &dataset.ID,
		DatasetName: &datasetName,
		UpdatedTs:   &now,
	}); err != nil {
		// Dataset 已创建但映射更新失败，记录警告但返回 DatasetID
		slog.Error("更新用户映射的 DatasetID 失败",
			slog.Int("userID", int(memosUserID)),
			slog.Any("error", err))
		return dataset.ID, nil
	}

	// 创建 Dataset 后，同时创建 Chat Assistant
	p.ensureAssistant(ctx, client, memosUserID, mapping.ID, dataset.ID)

	return dataset.ID, nil
}

// ensureAssistant 确保用户有对应的 Chat Assistant（最佳努力，失败不阻塞）
// 注意：创建时传空 dataset_ids，绕过 RAGFlow 对空 Dataset 的 chunk_num==0 校验。
// Dataset 关联将在内容同步完成后通过 UpdateChatAssistant 完成。
//
// LLM 选取优先级：
//  1. 用户已设置的 preferred_llm_id（stored in ragflow_user_mapping）
//  2. 系统默认 DefaultLLMID（来自环境变量 RAGFLOW_DEFAULT_LLM_ID）
//
// 前置保障：在尝试创建 Assistant 前，先调用 EnsureLLMConfig 确保该 Tenant
// 的 TenantLLM 表中已有对应的 LLM 配置记录，否则 RAGFlow 会以
// "`model_name` xxx doesn't exist" 拒绝请求。
func (p *Provisioner) ensureAssistant(ctx context.Context, client *Client, memosUserID int32, mappingID int32, datasetID string) {
	// 查询当前映射确认是否已有 AssistantID，同时获取 preferred_llm_id
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		slog.Warn("查询用户映射失败（ensureAssistant）", slog.Any("error", err))
		return
	}
	if mapping != nil && mapping.AssistantID != "" {
		return // 已有 Assistant
	}

	// ==================== 确定要使用的 LLM ID ====================

	// 优先使用用户已设置的偏好，降级到系统默认
	llmID := ""
	if mapping != nil && mapping.PreferredLLMID != "" {
		llmID = mapping.PreferredLLMID
		slog.Debug("ensureAssistant: 使用用户偏好 LLM",
			slog.Int("userID", int(memosUserID)),
			slog.String("llmID", llmID))
	} else if p.config.DefaultLLMID != "" {
		llmID = p.config.DefaultLLMID
		slog.Debug("ensureAssistant: 使用系统默认 LLM",
			slog.Int("userID", int(memosUserID)),
			slog.String("llmID", llmID))
	} else {
		slog.Warn("未配置 DefaultLLMID 且用户无 LLM 偏好，无法创建 Chat Assistant（对话功能将不可用）",
			slog.Int("userID", int(memosUserID)),
			slog.String("hint", "请设置环境变量 RAGFLOW_DEFAULT_LLM_ID，格式如 deepseek-chat@DeepSeek"))
		return
	}

	// ==================== 前置条件：确保 TenantLLM 表有该 LLM 记录 ====================

	// 调用 EnsureLLMConfig 确保百炼 LLM 配置已写入 RAGFlow TenantLLM 表。
	// 若 DashScopeAPIKey 未配置，此方法会静默跳过。
	// 若 LLM 已配置（LLMConfigured==true），此方法也会静默跳过（幂等）。
	if err := p.EnsureLLMConfig(ctx, memosUserID); err != nil {
		slog.Warn("ensureAssistant: EnsureLLMConfig 失败，Assistant 创建可能因此失败",
			slog.Int("userID", int(memosUserID)),
			slog.Any("error", err))
		// 继续尝试创建，让 RAGFlow 返回具体错误（而不是直接放弃）
	}

	// ==================== 创建 Assistant ====================

	assistantName := fmt.Sprintf("knowtree_assistant_%d", memosUserID)

	// 尝试创建 Assistant（传空 dataset_ids 绕过空 Dataset 校验，必须传 llm_id 和 rerank_id）
	assistant, err := client.CreateChatAssistant(ctx, &CreateChatAssistantRequest{
		Name:       assistantName,
		DatasetIDs: []string{},
		LLMID:      llmID,
		RerankID:   DefaultRerankModelName,
	})
	if err != nil {
		// 降级路径：如果是"重复名称"错误，说明 Assistant 已在 RAGFlow 中创建，
		// 但上次反序列化失败导致 ID 未保存。通过名称查询恢复。
		if strings.Contains(err.Error(), "Duplicat") {
			slog.Info("Chat Assistant 已存在（名称重复），尝试按名称查询恢复",
				slog.Int("userID", int(memosUserID)),
				slog.String("assistantName", assistantName))
			p.recoverAssistantByName(ctx, client, memosUserID, mappingID, assistantName)
			return
		}

		slog.Warn("创建 Chat Assistant 失败（非阻塞）",
			slog.Int("userID", int(memosUserID)),
			slog.String("llmID", llmID),
			slog.Any("error", err))
		return
	}

	slog.Info("为用户创建了 Chat Assistant",
		slog.Int("userID", int(memosUserID)),
		slog.String("assistantID", assistant.ID),
		slog.String("llmID", llmID))

	// 更新映射
	p.saveAssistantID(ctx, memosUserID, mappingID, assistant.ID)
}

// recoverAssistantByName 通过名称查询已有 Assistant 并保存其 ID
// 用于恢复"创建成功但反序列化失败"导致的半初始化状态
func (p *Provisioner) recoverAssistantByName(ctx context.Context, client *Client, memosUserID int32, mappingID int32, assistantName string) {
	assistants, err := client.ListChatAssistants(ctx, &ListOptions{
		Page:     1,
		PageSize: 10,
		Name:     assistantName,
	})
	if err != nil {
		slog.Warn("按名称查询 Chat Assistant 失败",
			slog.Int("userID", int(memosUserID)),
			slog.Any("error", err))
		return
	}

	// 精确匹配名称
	for _, a := range assistants {
		if a.Name == assistantName {
			slog.Info("通过名称查询恢复了 Chat Assistant",
				slog.Int("userID", int(memosUserID)),
				slog.String("assistantID", a.ID))
			p.saveAssistantID(ctx, memosUserID, mappingID, a.ID)
			return
		}
	}

	slog.Warn("按名称查询未找到匹配的 Chat Assistant",
		slog.Int("userID", int(memosUserID)),
		slog.String("assistantName", assistantName))
}

// saveAssistantID 将 AssistantID 写入映射表
func (p *Provisioner) saveAssistantID(ctx context.Context, memosUserID int32, mappingID int32, assistantID string) {
	now := time.Now().Unix()
	if err := p.store.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
		ID:          mappingID,
		AssistantID: &assistantID,
		UpdatedTs:   &now,
	}); err != nil {
		slog.Error("更新用户映射的 AssistantID 失败",
			slog.Int("userID", int(memosUserID)),
			slog.Any("error", err))
	}
}

// ==================== 认证配置流程 ====================

// provisionUser 为 Memos 用户配置 RAGFlow 账户
// 流程：
//  1. 查询现有映射 → 存在且 API Key 非空 → 直接创建 Client
//  2. 存在但 API Key 为空 → 执行登录 + 生成 API Key
//  3. 不存在或 ragflow_user_id 为空 → 完整注册流程
func (p *Provisioner) provisionUser(ctx context.Context, memosUserID int32, username string) (*Client, error) {
	// Step 1: 查询现有映射
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("查询用户映射失败: %w", err)
	}

	// 场景 A: 映射存在且 API Key 非空 → 直接返回
	if mapping != nil && mapping.APIKey != "" {
		slog.Debug("用户已配置 RAGFlow 账户，直接使用存储的 API Key",
			slog.Int("memosUserID", int(memosUserID)))
		return p.createClient(mapping.APIKey), nil
	}

	// 场景 B: 映射存在、已注册但 API Key 为空 → 需要登录获取 API Key
	if mapping != nil && mapping.RAGFlowUserID != "" && mapping.APIKey == "" {
		slog.Info("用户已注册 RAGFlow 但缺少 API Key，尝试登录获取",
			slog.Int("memosUserID", int(memosUserID)))
		return p.loginAndGenerateAPIKey(ctx, mapping)
	}

	// 场景 C: 映射不存在或 ragflow_user_id 为空 → 完整注册流程
	slog.Info("为用户创建 RAGFlow 账户",
		slog.Int("memosUserID", int(memosUserID)),
		slog.String("username", username))
	return p.registerNewUser(ctx, memosUserID, username, mapping)
}

// registerNewUser 执行完整的注册流程
func (p *Provisioner) registerNewUser(ctx context.Context, memosUserID int32, username string, existingMapping *store.RAGFlowUserMapping) (*Client, error) {
	// 生成凭据
	email, password := GenerateRAGFlowCredentials(memosUserID)

	// 尝试注册
	authResult, err := p.authClient.Register(ctx, &RegisterRequest{
		Email:    email,
		Nickname: username,
		Password: password,
	})

	// 如果用户已存在（例如之前注册成功但存储失败），降级到登录
	if err != nil {
		var authErr *AuthError
		if errors.As(err, &authErr) && authErr.Kind == AuthErrorUserExists {
			slog.Info("RAGFlow 用户已存在，降级到登录流程",
				slog.String("email", email))
			authResult, err = p.authClient.Login(ctx, &LoginRequest{
				Email:    email,
				Password: password,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("RAGFlow 认证失败: %w", err)
		}
	}

	// 生成 API Key
	apiKey, err := p.authClient.GenerateAPIKey(ctx, authResult.AuthToken)
	if err != nil {
		// 注册成功但 API Key 生成失败：先保存已有信息，API Key 留空，下次重试
		slog.Warn("生成 API Key 失败，保存已有信息待重试",
			slog.Int("memosUserID", int(memosUserID)),
			slog.String("error", err.Error()))
		_ = p.saveMapping(ctx, memosUserID, existingMapping, authResult.UserID, email, password, "")
		return nil, fmt.Errorf("生成 API Key 失败: %w", err)
	}

	// 保存完整映射
	if err := p.saveMapping(ctx, memosUserID, existingMapping, authResult.UserID, email, password, apiKey); err != nil {
		// 映射保存失败但 RAGFlow 账户已创建，返回 Client 但记录警告
		slog.Error("保存用户映射失败，RAGFlow 账户已创建但映射未持久化",
			slog.Int("memosUserID", int(memosUserID)),
			slog.String("error", err.Error()))
		return p.createClient(apiKey), nil
	}

	slog.Info("RAGFlow 用户配置完成",
		slog.Int("memosUserID", int(memosUserID)),
		slog.String("ragflowUserID", authResult.UserID),
		slog.String("email", email))

	return p.createClient(apiKey), nil
}

// loginAndGenerateAPIKey 登录已注册用户并生成 API Key
func (p *Provisioner) loginAndGenerateAPIKey(ctx context.Context, mapping *store.RAGFlowUserMapping) (*Client, error) {
	// 使用存储的凭据登录
	authResult, err := p.authClient.Login(ctx, &LoginRequest{
		Email:    mapping.RAGFlowEmail,
		Password: mapping.RAGFlowPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("RAGFlow 登录失败: %w", err)
	}

	// 生成 API Key
	apiKey, err := p.authClient.GenerateAPIKey(ctx, authResult.AuthToken)
	if err != nil {
		return nil, fmt.Errorf("生成 API Key 失败: %w", err)
	}

	// 更新映射中的 API Key
	now := time.Now().Unix()
	if err := p.store.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
		ID:        mapping.ID,
		APIKey:    &apiKey,
		UpdatedTs: &now,
	}); err != nil {
		slog.Error("更新 API Key 失败",
			slog.Int("mappingID", int(mapping.ID)),
			slog.String("error", err.Error()))
		// 即使持久化失败也返回可用的 Client
	}

	slog.Info("RAGFlow API Key 获取成功",
		slog.Int("memosUserID", int(mapping.UserID)),
		slog.String("email", mapping.RAGFlowEmail))

	return p.createClient(apiKey), nil
}

// ==================== 辅助方法 ====================

// saveMapping 保存或更新用户映射
func (p *Provisioner) saveMapping(ctx context.Context, memosUserID int32, existingMapping *store.RAGFlowUserMapping, ragflowUserID, email, password, apiKey string) error {
	if existingMapping != nil {
		// 更新现有映射
		now := time.Now().Unix()
		return p.store.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
			ID:              existingMapping.ID,
			RAGFlowUserID:   &ragflowUserID,
			RAGFlowEmail:    &email,
			RAGFlowPassword: &password,
			APIKey:          &apiKey,
			UpdatedTs:       &now,
		})
	}

	// 创建新映射
	_, err := p.store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
		UserID:          memosUserID,
		RAGFlowUserID:   ragflowUserID,
		RAGFlowEmail:    email,
		RAGFlowPassword: password,
		APIKey:          apiKey,
	})
	return err
}

// createClient 使用 API Key 创建 RAGFlow Client
func (p *Provisioner) createClient(apiKey string) *Client {
	return NewClient(&Config{
		BaseURL:    p.config.BaseURL,
		APIKey:     apiKey,
		Timeout:    p.config.Timeout,
		MaxRetries: p.config.MaxRetries,
	})
}

// ==================== LLM 配置 ====================

// EnsureLLMConfig 确保用户的 RAGFlow 账户已配置 LLM 提供商（百炼）
// 这是 P4 新增的方法，用于在用户首次使用 AI 对话时自动配置百炼模型。
//
// 流程：
// 1. 检查是否配置了 DashScopeAPIKey（系统级配置）
// 2. 使用用户凭据登录 RAGFlow 获取 AuthToken
// 3. 调用 Web API 配置 LLM 提供商（Tongyi-Qianwen）
// 4. 更新映射表标记 LLM 已配置
//
// 注意：此方法是幂等的，重复调用不会产生副作用。
func (p *Provisioner) EnsureLLMConfig(ctx context.Context, memosUserID int32) error {
	// 检查系统级配置
	if p.config.DashScopeAPIKey == "" {
		slog.Debug("未配置 DashScopeAPIKey，跳过 LLM 自动配置",
			slog.Int("userID", int(memosUserID)))
		return nil
	}

	// 查询用户映射
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return fmt.Errorf("查询用户映射失败: %w", err)
	}
	if mapping == nil {
		return fmt.Errorf("用户映射不存在，需先完成认证配置")
	}

	// 检查是否已配置 LLM（通过 LLMConfigured 字段判断）
	if mapping.LLMConfigured {
		slog.Debug("用户已配置 LLM，跳过",
			slog.Int("userID", int(memosUserID)))
		return nil
	}

	// 使用存储的凭据登录获取 AuthToken
	if mapping.RAGFlowEmail == "" || mapping.RAGFlowPassword == "" {
		return fmt.Errorf("用户凭据不完整，无法配置 LLM")
	}

	authResult, err := p.authClient.Login(ctx, &LoginRequest{
		Email:    mapping.RAGFlowEmail,
		Password: mapping.RAGFlowPassword,
	})
	if err != nil {
		return fmt.Errorf("登录 RAGFlow 失败: %w", err)
	}

	// 调用 Web API 配置 LLM 提供商
	err = p.authClient.SetLLMAPIKey(ctx, authResult.AuthToken, &SetLLMAPIKeyRequest{
		LLMFactory: "Tongyi-Qianwen",
		APIKey:     p.config.DashScopeAPIKey,
	})
	if err != nil {
		// 如果错误包含 "already" 或 "exist"，说明已配置，忽略错误
		if strings.Contains(strings.ToLower(err.Error()), "already") ||
			strings.Contains(strings.ToLower(err.Error()), "exist") {
			slog.Info("LLM 提供商已配置（忽略重复配置错误）",
				slog.Int("userID", int(memosUserID)))
		} else {
			return fmt.Errorf("配置 LLM 提供商失败: %w", err)
		}
	}

	// 设置租户默认模型（LLM、Embedding、ASR、VLM、Rerank、TTS）
	defaultModels := map[string]string{
		"llm_id":     p.config.DefaultLLMID,
		"embd_id":    DefaultEmbeddingModel,
		"asr_id":     DefaultASRModel,
		"img2txt_id": DefaultImage2TextModel,
		"rerank_id":  DefaultRerankModel,
		"tts_id":     DefaultTTSModel,
	}
	if err := p.authClient.SetTenantInfo(ctx, authResult.AuthToken, mapping.RAGFlowUserID, defaultModels); err != nil {
		slog.Warn("设置租户默认模型失败，LLM 提供商已配置但默认模型未设置",
			slog.Int("userID", int(memosUserID)),
			slog.Any("error", err))
		// 不阻塞流程，下次重试时会再次尝试
	} else {
		slog.Info("为用户设置了默认模型",
			slog.Int("userID", int(memosUserID)),
			slog.String("llm_id", p.config.DefaultLLMID),
			slog.String("embd_id", DefaultEmbeddingModel))
	}

	// 更新映射表标记 LLM 已配置
	now := time.Now().Unix()
	llmConfigured := true
	if err := p.store.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
		ID:            mapping.ID,
		LLMConfigured: &llmConfigured,
		UpdatedTs:     &now,
	}); err != nil {
		slog.Warn("更新 LLM 配置状态失败",
			slog.Int("userID", int(memosUserID)),
			slog.Any("error", err))
		// 不返回错误，LLM 已配置成功
	}

	slog.Info("为用户配置了 LLM 提供商和默认模型（Tongyi-Qianwen）",
		slog.Int("userID", int(memosUserID)))

	return nil
}

// UpdateUserAssistantLLM 更新用户 Assistant 的 LLM 模型
// 用于用户切换模型时同步更新 RAGFlow Assistant
func (p *Provisioner) UpdateUserAssistantLLM(ctx context.Context, memosUserID int32, modelName string) error {
	// 查询用户映射获取 AssistantID
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return fmt.Errorf("查询用户映射失败: %w", err)
	}
	if mapping == nil || mapping.AssistantID == "" {
		return fmt.Errorf("用户 Assistant 不存在")
	}
	if mapping.APIKey == "" {
		return fmt.Errorf("用户 API Key 不存在")
	}

	// 创建客户端并更新 Assistant
	client := p.createClient(mapping.APIKey)
	err = client.UpdateAssistantLLM(ctx, mapping.AssistantID, &UpdateAssistantLLMRequest{
		ModelName: modelName,
	})
	if err != nil {
		return fmt.Errorf("更新 Assistant LLM 失败: %w", err)
	}

	slog.Info("更新了用户 Assistant 的 LLM 模型",
		slog.Int("userID", int(memosUserID)),
		slog.String("assistantID", mapping.AssistantID),
		slog.String("modelName", modelName))

	return nil
}

// EnsureAssistantDatasetBinding 确保用户的 Chat Assistant 已绑定到其 Dataset。
// 如果 Assistant 已经绑定（dataset_ids 非空），则跳过。
// 在文档同步完成后调用，因为 RAGFlow 要求 dataset 的 chunk_num > 0。
func (p *Provisioner) EnsureAssistantDatasetBinding(ctx context.Context, memosUserID int32) error {
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return fmt.Errorf("查询用户映射失败: %w", err)
	}
	if mapping == nil || mapping.AssistantID == "" || mapping.DatasetID == "" {
		return nil // 尚未创建 Assistant 或 Dataset，跳过
	}
	if mapping.APIKey == "" {
		return fmt.Errorf("用户 API Key 不存在")
	}

	// 调用 RAGFlow API 绑定 Dataset 到 Assistant
	client := p.createClient(mapping.APIKey)
	err = client.UpdateAssistantDatasets(ctx, mapping.AssistantID, []string{mapping.DatasetID})
	if err != nil {
		return fmt.Errorf("绑定 Assistant ↔ Dataset 失败: %w", err)
	}

	slog.Info("成功绑定 Assistant ↔ Dataset",
		slog.Int("userID", int(memosUserID)),
		slog.String("assistantID", mapping.AssistantID),
		slog.String("datasetID", mapping.DatasetID))
	return nil
}

// UpdateUserAssistantDatasets 更新用户 Assistant 关联的 Dataset 列表
// 用于用户切换 Dataset 时同步更新 RAGFlow Assistant
func (p *Provisioner) UpdateUserAssistantDatasets(ctx context.Context, memosUserID int32, datasetIDs []string) error {
	// 查询用户映射获取 AssistantID
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return fmt.Errorf("查询用户映射失败: %w", err)
	}
	if mapping == nil || mapping.AssistantID == "" {
		return fmt.Errorf("用户 Assistant 不存在")
	}
	if mapping.APIKey == "" {
		return fmt.Errorf("用户 API Key 不存在")
	}

	// 创建客户端并更新 Assistant
	client := p.createClient(mapping.APIKey)
	err = client.UpdateAssistantDatasets(ctx, mapping.AssistantID, datasetIDs)
	if err != nil {
		return fmt.Errorf("更新 Assistant Datasets 失败: %w", err)
	}

	slog.Info("更新了用户 Assistant 的 Dataset 列表",
		slog.Int("userID", int(memosUserID)),
		slog.String("assistantID", mapping.AssistantID),
		slog.Any("datasetIDs", datasetIDs))

	return nil
}

// GetUserAssistantID 获取用户的 Assistant ID
func (p *Provisioner) GetUserAssistantID(ctx context.Context, memosUserID int32) (string, error) {
	mapping, err := p.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &memosUserID,
	})
	if err != nil {
		return "", fmt.Errorf("查询用户映射失败: %w", err)
	}
	if mapping == nil || mapping.AssistantID == "" {
		return "", fmt.Errorf("用户 Assistant 不存在")
	}
	return mapping.AssistantID, nil
}
