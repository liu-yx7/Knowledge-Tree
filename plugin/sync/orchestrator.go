package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pkg/errors"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// ==================== Orchestrator 定义 ====================

// UserGetter 获取用户信息的接口（避免 Orchestrator 直接依赖 store.User 查询）
type UserGetter interface {
	GetUser(ctx context.Context, find *store.FindUser) (*store.User, error)
}

// ResourceProvisioner 定义 Orchestrator 所需的 Provisioner 接口
// 使用接口解耦，便于测试和替换
type ResourceProvisioner interface {
	// EnsureUserResources 确保用户的全部 RAGFlow 资源就绪，返回 per-user Client 和 DatasetID
	EnsureUserResources(ctx context.Context, memosUserID int32, username string) (*ragflow.Client, string, error)
	// GetUserDatasetID 获取用户的 Dataset ID，不存在则自动创建
	GetUserDatasetID(ctx context.Context, memosUserID int32, username string) (string, error)
}

// Orchestrator 同步编排器
// 职责：协调 Memos 数据与 RAGFlow 之间的同步，是同步子系统的入口
// 通过 ResourceProvisioner 获取 per-user Client 和 DatasetID，
// 确保所有 RAGFlow API 调用都使用正确的用户认证。
type Orchestrator struct {
	store         *store.Store
	ragflowClient *ragflow.Client // 系统级 Client，仅用于健康检查和用户删除清理
	provisioner   ResourceProvisioner
	userGetter    UserGetter

	// 子模块
	stateTracker      *StateTracker
	healthChecker     *HealthChecker
	memoSyncer        *MemoSyncer
	attachmentSyncer  *AttachmentSyncer
	visibilityHandler *VisibilityHandler

	// 配置
	batchSize    int           // 批量同步大小
	syncInterval time.Duration // 定时同步间隔
}

// OrchestratorConfig 编排器配置
type OrchestratorConfig struct {
	BatchSize    int           // 默认 50
	SyncInterval time.Duration // 默认 5 分钟

	// 子模块配置
	StateTrackerConfig  *StateTrackerConfig
	HealthCheckerConfig *HealthCheckerConfig
}

// DefaultOrchestratorConfig 返回默认配置
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		BatchSize:           50,
		SyncInterval:        5 * time.Minute,
		StateTrackerConfig:  DefaultStateTrackerConfig(),
		HealthCheckerConfig: DefaultHealthCheckerConfig(),
	}
}

// NewOrchestrator 创建同步编排器
func NewOrchestrator(s *store.Store, client *ragflow.Client, cfg *OrchestratorConfig) *Orchestrator {
	if cfg == nil {
		cfg = DefaultOrchestratorConfig()
	}

	// 创建子模块
	stateTracker := NewStateTracker(s, cfg.StateTrackerConfig)
	healthChecker := NewHealthChecker(client, cfg.HealthCheckerConfig)
	memoSyncer := NewMemoSyncer(s, client, stateTracker)
	attachmentSyncer := NewAttachmentSyncer(s, client, stateTracker)
	visibilityHandler := NewVisibilityHandler(s, client, stateTracker)

	return &Orchestrator{
		store:             s,
		ragflowClient:     client,
		userGetter:        s, // Store 本身实现了 UserGetter 接口
		stateTracker:      stateTracker,
		healthChecker:     healthChecker,
		memoSyncer:        memoSyncer,
		attachmentSyncer:  attachmentSyncer,
		visibilityHandler: visibilityHandler,
		batchSize:         cfg.BatchSize,
		syncInterval:      cfg.SyncInterval,
	}
}

// SetProvisioner 注入 ResourceProvisioner
// Runner 在创建 Orchestrator 后调用此方法注入 Provisioner，
// 解决 Provisioner 和 Orchestrator 相互依赖的初始化顺序问题。
func (o *Orchestrator) SetProvisioner(p ResourceProvisioner) {
	o.provisioner = p
}

// ==================== 内部辅助：获取 per-user Client 和 DatasetID ====================

// getUsernameByID 通过 UserGetter 获取用户名
func (o *Orchestrator) getUsernameByID(ctx context.Context, userID int32) (string, error) {
	user, err := o.userGetter.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return "", fmt.Errorf("获取用户信息失败: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("用户 %d 不存在", userID)
	}
	return user.Username, nil
}

// ensureUserResourcesAndInjectClient 确保用户资源就绪，并将 per-user Client 注入子模块
// 返回 DatasetID。如果 Provisioner 不可用，降级到旧逻辑（使用系统级 Client）。
func (o *Orchestrator) ensureUserResourcesAndInjectClient(ctx context.Context, userID int32) (string, error) {
	if o.provisioner != nil {
		username, err := o.getUsernameByID(ctx, userID)
		if err != nil {
			return "", err
		}

		client, datasetID, err := o.provisioner.EnsureUserResources(ctx, userID, username)
		if err != nil {
			return "", errors.Wrap(err, "Provisioner 确保用户资源失败")
		}

		// 注入 per-user Client 到子模块
		o.memoSyncer.SetClient(client)
		o.attachmentSyncer.SetClient(client)
		o.visibilityHandler.SetClient(client)

		return datasetID, nil
	}

	// 降级：Provisioner 不可用，使用旧逻辑
	return o.ensureUserDatasetLegacy(ctx, userID)
}

// ensureUserDatasetLegacy 旧的 Dataset 创建逻辑（降级模式，使用系统级 Client）
func (o *Orchestrator) ensureUserDatasetLegacy(ctx context.Context, userID int32) (string, error) {
	mapping, err := o.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil {
		return "", errors.Wrap(err, "查询用户映射失败")
	}

	if mapping != nil && mapping.DatasetID != "" {
		return mapping.DatasetID, nil
	}

	datasetName := fmt.Sprintf("knowtree_user_%d", userID)
	dataset, err := o.ragflowClient.CreateDataset(ctx, &ragflow.CreateDatasetRequest{
		Name:        datasetName,
		Description: fmt.Sprintf("Knowtree 用户 %d 的知识库", userID),
		ChunkMethod: ragflow.ChunkMethodNaive,
	})
	if err != nil {
		return "", errors.Wrap(err, "创建 RAGFlow Dataset 失败（降级模式）")
	}

	slog.Info("为用户创建了新的 RAGFlow Dataset（降级模式）",
		slog.Int("userID", int(userID)),
		slog.String("datasetID", dataset.ID))

	now := time.Now().Unix()
	if mapping == nil {
		_, err = o.store.CreateRAGFlowUserMapping(ctx, &store.RAGFlowUserMapping{
			UserID:      userID,
			DatasetID:   dataset.ID,
			DatasetName: datasetName,
			CreatedTs:   now,
			UpdatedTs:   now,
		})
	} else {
		err = o.store.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
			ID:          mapping.ID,
			DatasetID:   &dataset.ID,
			DatasetName: &datasetName,
			UpdatedTs:   &now,
		})
	}
	if err != nil {
		return "", errors.Wrap(err, "保存用户映射失败")
	}

	return dataset.ID, nil
}

// ==================== 同步事件处理 ====================

// SyncEventType 同步事件类型
type SyncEventType string

const (
	SyncEventCreate SyncEventType = "create"
	SyncEventUpdate SyncEventType = "update"
	SyncEventDelete SyncEventType = "delete"
)

// SyncEvent 同步事件
type SyncEvent struct {
	Type        SyncEventType
	ContentType store.ContentType
	ContentUID  string
	UserID      int32
	Timestamp   time.Time
}

// HandleSyncEvent 处理同步事件
func (o *Orchestrator) HandleSyncEvent(ctx context.Context, event *SyncEvent) error {
	// 检查 RAGFlow 服务健康状态
	if !o.healthChecker.IsHealthy(ctx) {
		slog.Warn("RAGFlow 服务不可用，事件将在下次批量同步时处理",
			slog.String("eventType", string(event.Type)),
			slog.String("contentType", string(event.ContentType)),
			slog.String("contentUID", event.ContentUID))
		return nil // 不返回错误，让事件保持 pending 状态
	}

	switch event.Type {
	case SyncEventCreate, SyncEventUpdate:
		return o.handleCreateOrUpdate(ctx, event)
	case SyncEventDelete:
		return o.handleDelete(ctx, event)
	default:
		return errors.Errorf("未知的事件类型: %s", event.Type)
	}
}

// handleCreateOrUpdate 处理创建或更新事件
func (o *Orchestrator) handleCreateOrUpdate(ctx context.Context, event *SyncEvent) error {
	// 通过 Provisioner 确保用户资源就绪，并注入 per-user Client
	datasetID, err := o.ensureUserResourcesAndInjectClient(ctx, event.UserID)
	if err != nil {
		return errors.Wrap(err, "确保用户 Dataset 失败")
	}

	switch event.ContentType {
	case store.ContentTypeMemo:
		if err := o.memoSyncer.SyncMemo(ctx, event.ContentUID, datasetID); err != nil {
			o.healthChecker.RecordFailure(err)
			return err
		}
		o.healthChecker.RecordSuccess()
	case store.ContentTypeAttachment:
		if err := o.attachmentSyncer.SyncAttachment(ctx, event.ContentUID, datasetID); err != nil {
			o.healthChecker.RecordFailure(err)
			return err
		}
		o.healthChecker.RecordSuccess()
	}

	return nil
}

// handleDelete 处理删除事件
func (o *Orchestrator) handleDelete(ctx context.Context, event *SyncEvent) error {
	// 删除操作也需要 per-user Client（从 RAGFlow 删除文档需要认证）
	if o.provisioner != nil {
		username, err := o.getUsernameByID(ctx, event.UserID)
		if err == nil {
			client, _, _ := o.provisioner.EnsureUserResources(ctx, event.UserID, username)
			if client != nil {
				o.memoSyncer.SetClient(client)
				o.attachmentSyncer.SetClient(client)
			}
		}
	}

	switch event.ContentType {
	case store.ContentTypeMemo:
		return o.memoSyncer.DeleteMemoFromRAGFlow(ctx, event.ContentUID)
	case store.ContentTypeAttachment:
		return o.attachmentSyncer.DeleteAttachmentFromRAGFlow(ctx, event.ContentUID)
	}
	return nil
}

// ==================== 公开接口：用户 Dataset 管理 ====================

// EnsureUserDataset 确保用户有对应的 RAGFlow Dataset（公开接口，兼容旧调用方）
func (o *Orchestrator) EnsureUserDataset(ctx context.Context, userID int32) (string, error) {
	return o.ensureUserResourcesAndInjectClient(ctx, userID)
}

// GetUserDatasetID 获取用户的 Dataset ID
// 如果 Provisioner 可用，会自动创建缺失的资源；否则被动查询
func (o *Orchestrator) GetUserDatasetID(ctx context.Context, userID int32) (string, error) {
	if o.provisioner != nil {
		username, err := o.getUsernameByID(ctx, userID)
		if err != nil {
			return "", err
		}
		return o.provisioner.GetUserDatasetID(ctx, userID, username)
	}

	// 降级：被动查询
	mapping, err := o.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil {
		return "", errors.Wrap(err, "查询用户映射失败")
	}
	if mapping == nil {
		return "", errors.New("用户没有关联的 RAGFlow Dataset")
	}
	return mapping.DatasetID, nil
}

// ==================== 批量同步 ====================

// RunBatchSync 运行批量同步（处理所有 pending 和可重试的记录）
func (o *Orchestrator) RunBatchSync(ctx context.Context) error {
	// 检查 RAGFlow 服务健康状态
	if !o.healthChecker.IsHealthy(ctx) {
		slog.Warn("RAGFlow 服务不可用，跳过批量同步")
		return nil
	}

	slog.Info("开始批量同步...")
	startTime := time.Now()

	// 构建 datasetIDGetter：通过 Provisioner 获取 per-user Client 和 DatasetID
	datasetIDGetter := func(ownerID int32) (string, error) {
		return o.ensureUserResourcesAndInjectClient(ctx, ownerID)
	}

	// 1. 同步待处理的 Memo
	memoSuccess, memoFail, err := o.memoSyncer.SyncPendingMemos(ctx, datasetIDGetter, o.batchSize)
	if err != nil {
		slog.Error("批量同步 Memo 失败", slog.Any("error", err))
	}

	// 2. 同步待处理的附件
	attachmentSuccess, attachmentFail, err := o.attachmentSyncer.SyncPendingAttachments(ctx, datasetIDGetter, o.batchSize)
	if err != nil {
		slog.Error("批量同步附件失败", slog.Any("error", err))
	}

	// 3. 重试失败的记录
	retrySuccess, retryFail := o.retryFailedStates(ctx)

	duration := time.Since(startTime)
	slog.Info("批量同步完成",
		slog.Int("memoSuccess", memoSuccess),
		slog.Int("memoFail", memoFail),
		slog.Int("attachmentSuccess", attachmentSuccess),
		slog.Int("attachmentFail", attachmentFail),
		slog.Int("retrySuccess", retrySuccess),
		slog.Int("retryFail", retryFail),
		slog.Duration("duration", duration))

	return nil
}

// retryFailedStates 重试失败的状态
func (o *Orchestrator) retryFailedStates(ctx context.Context) (int, int) {
	retryableStates, err := o.stateTracker.ListRetryableStates(ctx, o.batchSize)
	if err != nil {
		slog.Error("获取可重试状态列表失败", slog.Any("error", err))
		return 0, 0
	}

	if len(retryableStates) == 0 {
		return 0, 0
	}

	successCount := 0
	failCount := 0

	for _, state := range retryableStates {
		datasetID, err := o.ensureUserResourcesAndInjectClient(ctx, state.OwnerID)
		if err != nil {
			slog.Error("获取用户 Dataset 失败（重试）",
				slog.Int("ownerID", int(state.OwnerID)),
				slog.Any("error", err))
			failCount++
			continue
		}

		var syncErr error
		switch state.ContentType {
		case store.ContentTypeMemo:
			syncErr = o.memoSyncer.SyncMemo(ctx, state.ContentUID, datasetID)
		case store.ContentTypeAttachment:
			syncErr = o.attachmentSyncer.SyncAttachment(ctx, state.ContentUID, datasetID)
		}

		if syncErr != nil {
			slog.Error("重试同步失败",
				slog.String("contentType", string(state.ContentType)),
				slog.String("contentUID", state.ContentUID),
				slog.Any("error", syncErr))
			if markErr := o.stateTracker.MarkAsFailed(ctx, state.ID, state.RetryCount, syncErr.Error()); markErr != nil {
				slog.Error("标记同步失败状态失败", slog.Any("error", markErr))
			}
			failCount++
			continue
		}

		successCount++
	}

	return successCount, failCount
}

// ==================== Visibility 变更处理 ====================

// HandleVisibilityChange 处理 visibility 变更
func (o *Orchestrator) HandleVisibilityChange(ctx context.Context, memoUID string, newVisibility store.Visibility) error {
	// 检查 RAGFlow 服务健康状态
	if !o.healthChecker.IsHealthy(ctx) {
		slog.Warn("RAGFlow 服务不可用，visibility 变更将在下次同步时生效",
			slog.String("memoUID", memoUID))
		return nil
	}

	// 获取 Memo 的 owner，以便注入 per-user Client
	memo, err := o.store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil || memo == nil {
		return errors.Wrap(err, "获取 Memo 失败")
	}

	// 注入 per-user Client
	if o.provisioner != nil {
		username, uErr := o.getUsernameByID(ctx, memo.CreatorID)
		if uErr == nil {
			client, _, _ := o.provisioner.EnsureUserResources(ctx, memo.CreatorID, username)
			if client != nil {
				o.visibilityHandler.SetClient(client)
			}
		}
	}

	// 更新 Memo 的 visibility
	if err := o.visibilityHandler.HandleVisibilityChange(ctx, memoUID, newVisibility); err != nil {
		return err
	}

	// 更新关联附件的 visibility
	if err := o.visibilityHandler.HandleAttachmentVisibilityChange(ctx, memoUID, newVisibility); err != nil {
		slog.Warn("更新附件 visibility 失败", slog.Any("error", err))
	}

	return nil
}

// ==================== 用户删除处理 ====================

// HandleUserDeletion 处理用户删除，清理 RAGFlow 资源
func (o *Orchestrator) HandleUserDeletion(ctx context.Context, userID int32) error {
	// 1. 获取用户映射
	mapping, err := o.store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &userID,
	})
	if err != nil {
		return errors.Wrap(err, "查询用户映射失败")
	}

	if mapping == nil {
		slog.Debug("用户没有 RAGFlow 资源，跳过清理", slog.Int("userID", int(userID)))
		return nil
	}

	// 用户删除时使用 per-user Client 清理资源（如果 Provisioner 可用）
	deleteClient := o.ragflowClient
	if o.provisioner != nil {
		username, uErr := o.getUsernameByID(ctx, userID)
		if uErr == nil {
			client, _, pErr := o.provisioner.EnsureUserResources(ctx, userID, username)
			if pErr == nil && client != nil {
				deleteClient = client
			}
		}
	}

	// 2. 删除 RAGFlow Dataset（会自动删除其中的所有文档）
	if mapping.DatasetID != "" {
		if err := deleteClient.DeleteDataset(ctx, mapping.DatasetID); err != nil {
			slog.Warn("删除 RAGFlow Dataset 失败",
				slog.String("datasetID", mapping.DatasetID),
				slog.Any("error", err))
		} else {
			slog.Info("已删除用户的 RAGFlow Dataset",
				slog.Int("userID", int(userID)),
				slog.String("datasetID", mapping.DatasetID))
		}
	}

	// 3. 删除 Chat Assistant（如果有）
	if mapping.AssistantID != "" {
		if err := deleteClient.DeleteChatAssistant(ctx, mapping.AssistantID); err != nil {
			slog.Warn("删除 RAGFlow Chat Assistant 失败",
				slog.String("assistantID", mapping.AssistantID),
				slog.Any("error", err))
		}
	}

	// 4. 删除用户映射
	if err := o.store.DeleteRAGFlowUserMapping(ctx, &store.DeleteRAGFlowUserMapping{
		UserID: &userID,
	}); err != nil {
		return errors.Wrap(err, "删除用户映射失败")
	}

	// 5. 删除所有同步状态
	if err := o.stateTracker.DeleteStatesByOwner(ctx, userID); err != nil {
		return errors.Wrap(err, "删除同步状态失败")
	}

	slog.Info("用户 RAGFlow 资源清理完成", slog.Int("userID", int(userID)))
	return nil
}

// ==================== 状态查询 ====================

// GetHealthStatus 获取健康状态
func (o *Orchestrator) GetHealthStatus() HealthStatus {
	return o.healthChecker.GetStatus()
}

// GetSyncStats 获取同步统计信息
func (o *Orchestrator) GetSyncStats(ctx context.Context) (*SyncStats, error) {
	pendingStatus := store.RAGFlowSyncStatusPending
	syncedStatus := store.RAGFlowSyncStatusSynced
	failedStatus := store.RAGFlowSyncStatusFailed
	skippedStatus := store.RAGFlowSyncStatusSkipped

	pending, _ := o.store.ListContentSyncStates(ctx, &store.FindContentSyncState{RAGFlowStatus: &pendingStatus})
	synced, _ := o.store.ListContentSyncStates(ctx, &store.FindContentSyncState{RAGFlowStatus: &syncedStatus})
	failed, _ := o.store.ListContentSyncStates(ctx, &store.FindContentSyncState{RAGFlowStatus: &failedStatus})
	skipped, _ := o.store.ListContentSyncStates(ctx, &store.FindContentSyncState{RAGFlowStatus: &skippedStatus})

	return &SyncStats{
		PendingCount: len(pending),
		SyncedCount:  len(synced),
		FailedCount:  len(failed),
		SkippedCount: len(skipped),
		TotalCount:   len(pending) + len(synced) + len(failed) + len(skipped),
	}, nil
}

// SyncStats 同步统计信息
type SyncStats struct {
	PendingCount int
	SyncedCount  int
	FailedCount  int
	SkippedCount int
	TotalCount   int
}

// ==================== 子模块访问器 ====================

// GetStateTracker 获取状态追踪器
func (o *Orchestrator) GetStateTracker() *StateTracker {
	return o.stateTracker
}

// GetHealthChecker 获取健康检查器
func (o *Orchestrator) GetHealthChecker() *HealthChecker {
	return o.healthChecker
}

// GetMemoSyncer 获取 Memo 同步器
func (o *Orchestrator) GetMemoSyncer() *MemoSyncer {
	return o.memoSyncer
}

// GetAttachmentSyncer 获取附件同步器
func (o *Orchestrator) GetAttachmentSyncer() *AttachmentSyncer {
	return o.attachmentSyncer
}

// GetVisibilityHandler 获取 visibility 处理器
func (o *Orchestrator) GetVisibilityHandler() *VisibilityHandler {
	return o.visibilityHandler
}

// GetSyncInterval 获取同步间隔
func (o *Orchestrator) GetSyncInterval() time.Duration {
	return o.syncInterval
}
