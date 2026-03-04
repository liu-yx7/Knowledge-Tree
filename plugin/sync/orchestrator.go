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
	// GetClientForUser 获取用户的 RAGFlow 客户端（仅认证，不创建 Dataset）
	GetClientForUser(ctx context.Context, memosUserID int32, username string) (*ragflow.Client, error)
	// EnsureUserResources 确保用户的 RAGFlow 认证和 Assistant 就绪
	// 注意：不再创建 legacy per-user Dataset，datasetID 始终返回空字符串
	EnsureUserResources(ctx context.Context, memosUserID int32, username string) (*ragflow.Client, string, error)
	// GetUserDatasetID 已废弃 — Dataset 现由 notebook 管理，此方法始终返回空
	// Deprecated: 使用 store.ListNotebooks 获取每个 notebook 的 DatasetID
	GetUserDatasetID(ctx context.Context, memosUserID int32, username string) (string, error)
	// EnsureAssistantDatasetBinding 确保用户的 Chat Assistant 已绑定到所有 notebook 的 Dataset
	// 从 notebooks 表收集所有 DatasetID 并更新 Assistant 绑定
	EnsureAssistantDatasetBinding(ctx context.Context, memosUserID int32) error
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

// resolveUserClient 仅获取用户的 per-user RAGFlow Client（不创建 Dataset）
// 用于同步场景：Dataset 由 notebook 决定，无需通过 Provisioner 创建 legacy per-user Dataset。
func (o *Orchestrator) resolveUserClient(ctx context.Context, userID int32) (*ragflow.Client, error) {
	if o.provisioner != nil {
		username, err := o.getUsernameByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		return o.provisioner.GetClientForUser(ctx, userID, username)
	}
	// 降级：Provisioner 不可用，使用系统级 Client
	return o.ragflowClient, nil
}

// resolveNotebookDatasetID 根据内容类型和 UID 查找其所属 notebook 的 RAGFlow Dataset ID。
// memo → memo.NotebookID → notebook.DatasetID
// attachment → attachment.MemoID → memo.NotebookID → notebook.DatasetID
// 如果 notebook 存在但 DatasetID 为空（创建时 RAGFlow 不可用），会尝试就地补偿创建。
func (o *Orchestrator) resolveNotebookDatasetID(ctx context.Context, contentType store.ContentType, contentUID string) (string, error) {
	switch contentType {
	case store.ContentTypeMemo:
		memo, err := o.store.GetMemo(ctx, &store.FindMemo{UID: &contentUID})
		if err != nil {
			return "", fmt.Errorf("获取 Memo 失败: %w", err)
		}
		if memo == nil {
			return "", fmt.Errorf("Memo %s 不存在", contentUID)
		}
		if memo.NotebookID == nil {
			return "", fmt.Errorf("Memo %s 未关联 notebook", contentUID)
		}
		nb, err := o.store.GetNotebook(ctx, &store.FindNotebook{ID: memo.NotebookID})
		if err != nil {
			return "", fmt.Errorf("获取 Notebook 失败: %w", err)
		}
		if nb == nil {
			return "", fmt.Errorf("Notebook %d 不存在", *memo.NotebookID)
		}
		return o.ensureNotebookHasDataset(ctx, nb, memo.CreatorID)

	case store.ContentTypeAttachment:
		att, err := o.store.GetAttachment(ctx, &store.FindAttachment{UID: &contentUID})
		if err != nil {
			return "", fmt.Errorf("获取附件失败: %w", err)
		}
		if att == nil {
			return "", fmt.Errorf("附件 %s 不存在", contentUID)
		}
		if att.MemoID == nil {
			return "", fmt.Errorf("附件 %s 未关联 memo", contentUID)
		}
		memo, err := o.store.GetMemo(ctx, &store.FindMemo{ID: att.MemoID})
		if err != nil {
			return "", fmt.Errorf("获取附件关联的 Memo 失败: %w", err)
		}
		if memo == nil {
			return "", fmt.Errorf("附件关联的 Memo 不存在")
		}
		if memo.NotebookID == nil {
			return "", fmt.Errorf("附件关联的 Memo 未关联 notebook")
		}
		nb, err := o.store.GetNotebook(ctx, &store.FindNotebook{ID: memo.NotebookID})
		if err != nil {
			return "", fmt.Errorf("获取 Notebook 失败: %w", err)
		}
		if nb == nil {
			return "", fmt.Errorf("Notebook %d 不存在", *memo.NotebookID)
		}
		return o.ensureNotebookHasDataset(ctx, nb, memo.CreatorID)
	}

	return "", fmt.Errorf("未知的内容类型: %s", contentType)
}

// ensureNotebookHasDataset 确保 notebook 有关联的 RAGFlow Dataset。
// 如果 DatasetID 已存在，直接返回。
// 如果 DatasetID 为空（创建时 RAGFlow 不可用），尝试就地补偿创建。
func (o *Orchestrator) ensureNotebookHasDataset(ctx context.Context, nb *store.Notebook, ownerID int32) (string, error) {
	if nb.DatasetID != "" {
		return nb.DatasetID, nil
	}

	// 补偿：notebook 没有 dataset，尝试创建
	if o.provisioner == nil {
		return "", fmt.Errorf("notebook %d 没有关联的 RAGFlow dataset 且 Provisioner 不可用", nb.ID)
	}

	username, err := o.getUsernameByID(ctx, ownerID)
	if err != nil {
		return "", fmt.Errorf("补偿创建 dataset: 获取用户名失败: %w", err)
	}

	client, err := o.provisioner.GetClientForUser(ctx, ownerID, username)
	if err != nil {
		return "", fmt.Errorf("补偿创建 dataset: 获取用户客户端失败: %w", err)
	}

	datasetName := fmt.Sprintf("knowtree_nb_%s", nb.UID[:8])
	dataset, err := client.CreateDataset(ctx, &ragflow.CreateDatasetRequest{
		Name:        datasetName,
		Description: nb.Title,
		ChunkMethod: ragflow.ChunkMethodNaive,
	})
	if err != nil {
		return "", fmt.Errorf("补偿创建 dataset 失败: %w", err)
	}

	// 回填 DatasetID 到 notebook
	if updateErr := o.store.UpdateNotebook(ctx, &store.UpdateNotebook{
		ID:        nb.ID,
		DatasetID: &dataset.ID,
	}); updateErr != nil {
		slog.Warn("补偿更新 notebook datasetID 失败",
			slog.Int("notebookID", int(nb.ID)),
			slog.Any("error", updateErr))
		// dataset 已创建，返回 ID 即使 DB 更新失败
		return dataset.ID, nil
	}

	slog.Info("补偿创建 notebook dataset 成功",
		slog.Int("notebookID", int(nb.ID)),
		slog.String("datasetID", dataset.ID),
		slog.String("datasetName", datasetName))

	return dataset.ID, nil
}

// resolveUserResources 确保用户资源就绪，返回 per-user Client 和 DatasetID
// Deprecated: 此方法保留用于 EnsureUserDataset / GetUserDatasetID 等旧接口的降级路径。
// 新代码应使用 resolveUserClient + resolveNotebookDatasetID。
func (o *Orchestrator) resolveUserResources(ctx context.Context, userID int32) (*ragflow.Client, string, error) {
	if o.provisioner != nil {
		username, err := o.getUsernameByID(ctx, userID)
		if err != nil {
			return nil, "", err
		}

		client, datasetID, err := o.provisioner.EnsureUserResources(ctx, userID, username)
		if err != nil {
			return nil, "", errors.Wrap(err, "Provisioner 确保用户资源失败")
		}

		return client, datasetID, nil
	}

	// 降级：Provisioner 不可用，使用系统级 Client
	datasetID, err := o.ensureUserDatasetLegacy(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	return o.ragflowClient, datasetID, nil
}

// ensureUserDatasetLegacy 旧的 Dataset 创建逻辑（降级模式，使用系统级 Client）
// Deprecated: 仅在 Provisioner 不可用时作为降级路径。新架构中 Dataset 由 notebook 管理。
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
	slog.Info("收到同步事件",
		slog.String("eventType", string(event.Type)),
		slog.String("contentType", string(event.ContentType)),
		slog.String("contentUID", event.ContentUID),
		slog.Int("userID", int(event.UserID)))

	// 对于 Create/Update 事件，先持久化 pending 状态，确保事件不丢失
	// 使用 EnsurePendingState：已存在的记录不会被覆盖（非破坏性）
	if event.Type == SyncEventCreate || event.Type == SyncEventUpdate {
		if _, err := o.stateTracker.EnsurePendingState(ctx, event.ContentType, event.ContentUID, event.UserID); err != nil {
			slog.Warn("创建 pending 状态失败，继续尝试同步",
				slog.String("contentUID", event.ContentUID),
				slog.Any("error", err))
		}
	}

	// 检查 RAGFlow 服务健康状态
	if !o.healthChecker.IsHealthy(ctx) {
		slog.Warn("RAGFlow 服务不可用，事件已持久化，将在下次批量同步时处理",
			slog.String("eventType", string(event.Type)),
			slog.String("contentType", string(event.ContentType)),
			slog.String("contentUID", event.ContentUID))
		return nil // 事件已持久化为 pending，批量同步会处理
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
	// 获取 per-user Client（仅认证，不创建 legacy Dataset）
	client, err := o.resolveUserClient(ctx, event.UserID)
	if err != nil {
		return errors.Wrap(err, "获取用户 Client 失败")
	}

	// 从 notebook 解析目标 Dataset ID
	datasetID, err := o.resolveNotebookDatasetID(ctx, event.ContentType, event.ContentUID)
	if err != nil {
		slog.Warn("无法从 notebook 解析 dataset，跳过同步",
			slog.String("contentType", string(event.ContentType)),
			slog.String("contentUID", event.ContentUID),
			slog.Any("error", err))
		return nil
	}
	if datasetID == "" {
		slog.Warn("notebook 没有关联的 RAGFlow dataset，跳过同步",
			slog.String("contentUID", event.ContentUID))
		return nil
	}

	switch event.ContentType {
	case store.ContentTypeMemo:
		if err := o.memoSyncer.SyncMemo(ctx, event.ContentUID, datasetID, client); err != nil {
			o.healthChecker.RecordFailure(err)
			return err
		}
		o.healthChecker.RecordSuccess()
	case store.ContentTypeAttachment:
		if err := o.attachmentSyncer.SyncAttachment(ctx, event.ContentUID, datasetID, client); err != nil {
			o.healthChecker.RecordFailure(err)
			return err
		}
		o.healthChecker.RecordSuccess()
	}

	return nil
}

// handleDelete 处理删除事件
func (o *Orchestrator) handleDelete(ctx context.Context, event *SyncEvent) error {
	// 获取 per-user Client（仅认证）
	client, err := o.resolveUserClient(ctx, event.UserID)
	if err != nil {
		slog.Warn("获取用户 Client 失败，使用系统级 Client", slog.Any("error", err))
		client = o.ragflowClient
	}

	switch event.ContentType {
	case store.ContentTypeMemo:
		return o.memoSyncer.DeleteMemoFromRAGFlow(ctx, event.ContentUID, client)
	case store.ContentTypeAttachment:
		return o.attachmentSyncer.DeleteAttachmentFromRAGFlow(ctx, event.ContentUID, client)
	}
	return nil
}

// ==================== 公开接口：用户 Dataset 管理 ====================

// EnsureUserDataset 确保用户有对应的 RAGFlow Dataset（公开接口，兼容旧调用方）
// Deprecated: Dataset 现由 notebook 管理，此方法仅保留向后兼容。
func (o *Orchestrator) EnsureUserDataset(ctx context.Context, userID int32) (string, error) {
	_, datasetID, err := o.resolveUserResources(ctx, userID)
	return datasetID, err
}

// GetUserDatasetID 获取用户的 Dataset ID
// Deprecated: Dataset 现由 notebook 管理。使用 store.ListNotebooks 获取每个 notebook 的 DatasetID。
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

	// 0. 先发现未被追踪的内容，补齐 pending 状态
	discovered := o.discoverUntrackedContent(ctx)

	// 构建 resourceGetter：通过 notebook 解析目标 Dataset ID
	resourceGetter := func(ownerID int32, contentType store.ContentType, contentUID string) (*ragflow.Client, string, error) {
		client, err := o.resolveUserClient(ctx, ownerID)
		if err != nil {
			return nil, "", err
		}
		datasetID, err := o.resolveNotebookDatasetID(ctx, contentType, contentUID)
		if err != nil {
			return nil, "", err
		}
		if datasetID == "" {
			return nil, "", fmt.Errorf("notebook 没有关联的 RAGFlow dataset")
		}
		return client, datasetID, nil
	}

	// 1. 同步待处理的 Memo
	memoSuccess, memoFail, err := o.memoSyncer.SyncPendingMemos(ctx, resourceGetter, o.batchSize)
	if err != nil {
		slog.Error("批量同步 Memo 失败", slog.Any("error", err))
	}

	// 2. 同步待处理的附件
	attachmentSuccess, attachmentFail, err := o.attachmentSyncer.SyncPendingAttachments(ctx, resourceGetter, o.batchSize)
	if err != nil {
		slog.Error("批量同步附件失败", slog.Any("error", err))
	}

	// 3. 重试失败的记录
	retrySuccess, retryFail := o.retryFailedStates(ctx)

	// 4. 如果有成功同步的内容，尝试绑定 Assistant ↔ Dataset
	totalSynced := memoSuccess + attachmentSuccess + retrySuccess
	bindSuccess, bindFail := 0, 0
	if totalSynced > 0 {
		bindSuccess, bindFail = o.ensureAssistantDatasetBindings(ctx)
	}

	duration := time.Since(startTime)
	slog.Info("批量同步完成",
		slog.Int("discovered", discovered),
		slog.Int("memoSuccess", memoSuccess),
		slog.Int("memoFail", memoFail),
		slog.Int("attachmentSuccess", attachmentSuccess),
		slog.Int("attachmentFail", attachmentFail),
		slog.Int("retrySuccess", retrySuccess),
		slog.Int("retryFail", retryFail),
		slog.Int("bindSuccess", bindSuccess),
		slog.Int("bindFail", bindFail),
		slog.Duration("duration", duration))

	return nil
}

// ensureAssistantDatasetBindings 遍历所有有 AssistantID 的用户，
// 确保 Assistant 已绑定到对应的 Dataset。
// 只有在文档已上传并开始解析后才能成功（RAGFlow 要求 chunk_num > 0）。
func (o *Orchestrator) ensureAssistantDatasetBindings(ctx context.Context) (int, int) {
	if o.provisioner == nil {
		return 0, 0
	}

	// 获取所有同步状态中涉及的不同用户 ID
	syncedStatus := store.RAGFlowSyncStatusSynced
	states, err := o.store.ListContentSyncStates(ctx, &store.FindContentSyncState{
		RAGFlowStatus: &syncedStatus,
	})
	if err != nil {
		slog.Error("查询已同步状态失败", slog.Any("error", err))
		return 0, 0
	}

	// 收集不同的 ownerID
	ownerSet := make(map[int32]struct{})
	for _, s := range states {
		ownerSet[s.OwnerID] = struct{}{}
	}

	successCount, failCount := 0, 0
	for ownerID := range ownerSet {
		if err := o.provisioner.EnsureAssistantDatasetBinding(ctx, ownerID); err != nil {
			slog.Warn("绑定 Assistant ↔ Dataset 失败",
				slog.Int("ownerID", int(ownerID)),
				slog.Any("error", err))
			failCount++
		} else {
			successCount++
		}
	}

	return successCount, failCount
}

// discoverUntrackedContent 扫描系统中没有 sync state 记录的 memo 和 attachment，
// 为它们创建 pending 状态，并将已有的 failed/skipped 状态重置为 pending。
// 这是一种 catch-up 机制，确保所有内容最终都能被同步到 RAGFlow。
func (o *Orchestrator) discoverUntrackedContent(ctx context.Context) int {
	newlyCreated := 0

	// 1. 扫描未追踪的 memo（没有 sync state 记录的）
	normalStatus := store.Normal
	memos, err := o.store.ListMemos(ctx, &store.FindMemo{
		RowStatus: &normalStatus,
	})
	if err != nil {
		slog.Error("扫描 memo 失败", slog.Any("error", err))
	} else {
		for _, memo := range memos {
			created, err := o.stateTracker.EnsurePendingState(ctx, store.ContentTypeMemo, memo.UID, memo.CreatorID)
			if err != nil {
				slog.Warn("为 memo 创建 pending 状态失败",
					slog.String("memoUID", memo.UID),
					slog.Any("error", err))
			} else if created {
				newlyCreated++
			}
		}
	}

	// 2. 扫描未追踪的 attachment
	attachments, err := o.store.ListAttachments(ctx, &store.FindAttachment{})
	if err != nil {
		slog.Error("扫描 attachment 失败", slog.Any("error", err))
	} else {
		for _, att := range attachments {
			created, err := o.stateTracker.EnsurePendingState(ctx, store.ContentTypeAttachment, att.UID, att.CreatorID)
			if err != nil {
				slog.Warn("为 attachment 创建 pending 状态失败",
					slog.String("attachmentUID", att.UID),
					slog.Any("error", err))
			} else if created {
				newlyCreated++
			}
		}
	}

	// 3. 将已有的 failed/skipped 状态重置为 pending
	resetCount, err := o.stateTracker.ResetNonPendingStates(ctx)
	if err != nil {
		slog.Error("重置 failed/skipped 状态失败", slog.Any("error", err))
	}

	totalRecovered := newlyCreated + resetCount
	if totalRecovered > 0 {
		slog.Info("内容同步状态恢复完成",
			slog.Int("newlyCreated", newlyCreated),
			slog.Int("resetFromFailedOrSkipped", resetCount))
	}

	return totalRecovered
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
		client, err := o.resolveUserClient(ctx, state.OwnerID)
		if err != nil {
			slog.Error("获取用户 Client 失败（重试）",
				slog.Int("ownerID", int(state.OwnerID)),
				slog.Any("error", err))
			failCount++
			continue
		}

		datasetID, err := o.resolveNotebookDatasetID(ctx, state.ContentType, state.ContentUID)
		if err != nil {
			slog.Warn("无法从 notebook 解析 dataset（重试），跳过",
				slog.String("contentUID", state.ContentUID),
				slog.Any("error", err))
			failCount++
			continue
		}

		var syncErr error
		switch state.ContentType {
		case store.ContentTypeMemo:
			syncErr = o.memoSyncer.SyncMemo(ctx, state.ContentUID, datasetID, client)
		case store.ContentTypeAttachment:
			syncErr = o.attachmentSyncer.SyncAttachment(ctx, state.ContentUID, datasetID, client)
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

	// 获取 Memo 的 owner，以便获取 per-user Client
	memo, err := o.store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil || memo == nil {
		return errors.Wrap(err, "获取 Memo 失败")
	}

	// 获取 per-user Client（仅认证，不创建 legacy Dataset）
	client, err := o.resolveUserClient(ctx, memo.CreatorID)
	if err != nil {
		slog.Warn("获取用户 Client 失败，使用系统级 Client", slog.Any("error", err))
		client = o.ragflowClient
	}

	// 更新 Memo 的 visibility
	if err := o.visibilityHandler.HandleVisibilityChange(ctx, memoUID, newVisibility, client); err != nil {
		return err
	}

	// 更新关联附件的 visibility
	if err := o.visibilityHandler.HandleAttachmentVisibilityChange(ctx, memoUID, newVisibility, client); err != nil {
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

	// 用户删除时使用 per-user Client 清理资源
	deleteClient, err := o.resolveUserClient(ctx, userID)
	if err != nil {
		slog.Warn("获取用户 Client 失败，使用系统级 Client 清理", slog.Any("error", err))
		deleteClient = o.ragflowClient
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
