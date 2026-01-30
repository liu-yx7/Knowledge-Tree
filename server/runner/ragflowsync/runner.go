package ragflowsync

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/usememos/memos/plugin/ragflow"
	pluginsync "github.com/usememos/memos/plugin/sync"
	"github.com/usememos/memos/store"
)

// ==================== Runner 定义 ====================

// Runner RAGFlow 同步运行器
// 职责：协调后台同步任务，响应实时事件
type Runner struct {
	store         *store.Store
	config        *ragflow.Config
	ragflowClient *ragflow.Client
	orchestrator  *pluginsync.Orchestrator

	// 运行状态
	running atomic.Bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// 配置
	syncInterval time.Duration
	batchSize    int
}

// NewRunner 创建同步运行器
func NewRunner(s *store.Store, config *ragflow.Config) *Runner {
	if config == nil {
		return nil
	}

	// 创建 RAGFlow 客户端
	client := ragflow.NewClient(config)

	// 创建 Orchestrator
	orchestratorConfig := pluginsync.DefaultOrchestratorConfig()
	orchestrator := pluginsync.NewOrchestrator(s, client, orchestratorConfig)

	return &Runner{
		store:         s,
		config:        config,
		ragflowClient: client,
		orchestrator:  orchestrator,
		stopCh:        make(chan struct{}),
		syncInterval:  5 * time.Minute, // 默认 5 分钟同步一次
		batchSize:     50,              // 默认每批 50 条
	}
}

// ==================== 生命周期管理 ====================

// Run 启动运行器主循环
func (r *Runner) Run(ctx context.Context) {
	if r == nil || r.config == nil {
		return
	}

	if !r.running.CompareAndSwap(false, true) {
		slog.Warn("RAGFlow sync runner is already running")
		return
	}
	defer r.running.Store(false)

	slog.Info("RAGFlow sync runner started",
		slog.Duration("interval", r.syncInterval),
		slog.Int("batchSize", r.batchSize))

	// 启动时执行一次完整同步
	r.runSyncCycle(ctx)

	ticker := time.NewTicker(r.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("RAGFlow sync runner stopping due to context cancellation")
			return
		case <-r.stopCh:
			slog.Info("RAGFlow sync runner stopping due to stop signal")
			return
		case <-ticker.C:
			r.runSyncCycle(ctx)
		}
	}
}

// Stop 停止运行器
func (r *Runner) Stop() {
	if r == nil {
		return
	}
	close(r.stopCh)
	r.wg.Wait()
}

// IsRunning 检查运行器是否正在运行
func (r *Runner) IsRunning() bool {
	if r == nil {
		return false
	}
	return r.running.Load()
}

// ==================== 同步循环 ====================

// runSyncCycle 执行一次同步循环
func (r *Runner) runSyncCycle(ctx context.Context) {
	if r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	defer r.wg.Done()

	startTime := time.Now()
	slog.Debug("Starting RAGFlow sync cycle")

	// 执行批量同步
	if err := r.orchestrator.RunBatchSync(ctx); err != nil {
		slog.Error("RAGFlow sync cycle failed", slog.Any("error", err))
		return
	}

	duration := time.Since(startTime)
	slog.Info("RAGFlow sync cycle completed", slog.Duration("duration", duration))
}

// TriggerSync 手动触发一次同步
func (r *Runner) TriggerSync(ctx context.Context) error {
	if r == nil || r.orchestrator == nil {
		return nil
	}

	r.wg.Add(1)
	defer r.wg.Done()

	slog.Info("Manual sync triggered")
	r.runSyncCycle(ctx)
	return nil
}

// ==================== 实时事件处理 ====================

// OnMemoCreated 处理 Memo 创建事件
func (r *Runner) OnMemoCreated(ctx context.Context, memo *store.Memo) {
	if r == nil || r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		event := &pluginsync.SyncEvent{
			Type:        pluginsync.SyncEventCreate,
			ContentType: store.ContentTypeMemo,
			ContentUID:  memo.UID,
			UserID:      memo.CreatorID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(ctx, event); err != nil {
			slog.Error("处理 Memo 创建事件失败",
				slog.String("memoUID", memo.UID),
				slog.Any("error", err))
		}
	}()
}

// OnMemoUpdated 处理 Memo 更新事件
func (r *Runner) OnMemoUpdated(ctx context.Context, memo *store.Memo) {
	if r == nil || r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		event := &pluginsync.SyncEvent{
			Type:        pluginsync.SyncEventUpdate,
			ContentType: store.ContentTypeMemo,
			ContentUID:  memo.UID,
			UserID:      memo.CreatorID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(ctx, event); err != nil {
			slog.Error("处理 Memo 更新事件失败",
				slog.String("memoUID", memo.UID),
				slog.Any("error", err))
		}
	}()
}

// OnMemoDeleted 处理 Memo 删除事件
func (r *Runner) OnMemoDeleted(ctx context.Context, memoUID string) {
	if r == nil || r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		event := &pluginsync.SyncEvent{
			Type:        pluginsync.SyncEventDelete,
			ContentType: store.ContentTypeMemo,
			ContentUID:  memoUID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(ctx, event); err != nil {
			slog.Error("处理 Memo 删除事件失败",
				slog.String("memoUID", memoUID),
				slog.Any("error", err))
		}
	}()
}

// OnMemoVisibilityChanged 处理 Memo 可见性变更事件
func (r *Runner) OnMemoVisibilityChanged(ctx context.Context, memoUID string, visibility store.Visibility) {
	if r == nil || r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		if err := r.orchestrator.HandleVisibilityChange(ctx, memoUID, visibility); err != nil {
			slog.Error("处理 Memo 可见性变更事件失败",
				slog.String("memoUID", memoUID),
				slog.Any("error", err))
		}
	}()
}

// OnAttachmentCreated 处理附件创建事件
func (r *Runner) OnAttachmentCreated(ctx context.Context, attachment *store.Attachment) {
	if r == nil || r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		event := &pluginsync.SyncEvent{
			Type:        pluginsync.SyncEventCreate,
			ContentType: store.ContentTypeAttachment,
			ContentUID:  attachment.UID,
			UserID:      attachment.CreatorID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(ctx, event); err != nil {
			slog.Error("处理附件创建事件失败",
				slog.String("attachmentUID", attachment.UID),
				slog.Any("error", err))
		}
	}()
}

// OnAttachmentDeleted 处理附件删除事件
func (r *Runner) OnAttachmentDeleted(ctx context.Context, attachmentUID string) {
	if r == nil || r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		event := &pluginsync.SyncEvent{
			Type:        pluginsync.SyncEventDelete,
			ContentType: store.ContentTypeAttachment,
			ContentUID:  attachmentUID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(ctx, event); err != nil {
			slog.Error("处理附件删除事件失败",
				slog.String("attachmentUID", attachmentUID),
				slog.Any("error", err))
		}
	}()
}

// OnUserDeleted 处理用户删除事件
func (r *Runner) OnUserDeleted(ctx context.Context, userID int32) {
	if r == nil || r.orchestrator == nil {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		if err := r.orchestrator.HandleUserDeletion(ctx, userID); err != nil {
			slog.Error("处理用户删除事件失败",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
		}
	}()
}

// ==================== 状态查询 ====================

// GetHealthStatus 获取健康状态
func (r *Runner) GetHealthStatus() *pluginsync.HealthStatus {
	if r == nil || r.orchestrator == nil {
		return nil
	}
	status := r.orchestrator.GetHealthStatus()
	return &status
}

// GetSyncStats 获取同步统计
func (r *Runner) GetSyncStats(ctx context.Context) (*pluginsync.SyncStats, error) {
	if r == nil || r.orchestrator == nil {
		return nil, nil
	}
	return r.orchestrator.GetSyncStats(ctx)
}

// GetOrchestrator 获取 Orchestrator（用于服务层访问）
func (r *Runner) GetOrchestrator() *pluginsync.Orchestrator {
	if r == nil {
		return nil
	}
	return r.orchestrator
}
