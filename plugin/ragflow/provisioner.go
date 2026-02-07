package ragflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/usememos/memos/store"
)

// ==================== Provisioner 定义 ====================

// Provisioner RAGFlow 用户自动配置器
// 职责：协调 AuthClient（注册/登录/API Key 生成）和 Store（持久化映射），
// 为 Memos 用户透明地配置 RAGFlow 账户，实现"用户无感知"的 RAG 功能接入。
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

// InvalidateCache 清除指定用户的缓存（用于 API Key 失效后重新配置）
func (p *Provisioner) InvalidateCache(memosUserID int32) {
	p.mu.Lock()
	delete(p.clientCache, memosUserID)
	p.mu.Unlock()
}

// ==================== 内部配置流程 ====================

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
