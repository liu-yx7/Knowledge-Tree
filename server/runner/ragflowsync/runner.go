// Package ragflowsync 提供 RAGFlow 后台同步任务
// 职责：定期运行批量同步，处理待同步和失败重试的内容
package ragflowsync

import (
	"context"
	"log/slog"
	"time"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/plugin/sync"
	"github.com/usememos/memos/store"
)

// ==================== Runner 定义 ====================

// Runner RAGFlow 同步后台任务
// 职责：
// 1. 定期执行批量同步
// 2. 启动后台健康检查
// 3. 提供手动触发同步的能力
type Runner struct {
	orchestrator *sync.Orchestrator
	store        *store.Store

	// 控制
	cancelFunc context.CancelFunc
	running    bool
}

// NewRunner 创建 RAGFlow 同步 Runner
// 如果 RAGFlow 未配置，返回 nil
func NewRunner(s *store.Store, ragflowConfig *ragflow.Config) *Runner {
	// 检查 RAGFlow 配置是否有效
	if ragflowConfig == nil || ragflowConfig.BaseURL == "" || ragflowConfig.APIKey == "" {
		slog.Info("RAGFlow 未配置，同步功能已禁用")
		return nil
	}

	// 验证配置
	if err := ragflowConfig.Validate(); err != nil {
		slog.Error("RAGFlow 配置无效，同步功能已禁用", slog.Any("error", err))
		return nil
	}

	// 创建 RAGFlow 客户端
	client := ragflow.NewClient(ragflowConfig.WithDefaults())

	// 创建编排器
	orchestrator := sync.NewOrchestrator(s, client, nil)

	return &Runner{
		orchestrator: orchestrator,
		store:        s,
	}
}

// ==================== 生命周期管理 ====================

// Start 启动后台同步任务
func (r *Runner) Start(ctx context.Context) {
	if r == nil {
		return
	}

	if r.running {
		slog.Warn("RAGFlow 同步任务已在运行中")
		return
	}

	// 创建可取消的上下文
	runCtx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel
	r.running = true

	slog.Info("RAGFlow 同步后台任务启动",
		slog.Duration("syncInterval", r.orchestrator.GetSyncInterval()))

	// 启动后台健康检查
	healthCancelFunc := r.orchestrator.GetHealthChecker().StartBackgroundCheck(runCtx)

	// 启动定时同步任务
	go r.runSyncLoop(runCtx, healthCancelFunc)
}

// Stop 停止后台同步任务
func (r *Runner) Stop() {
	if r == nil || !r.running {
		return
	}

	slog.Info("正在停止 RAGFlow 同步后台任务...")
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	r.running = false
	slog.Info("RAGFlow 同步后台任务已停止")
}

// IsRunning 返回任务是否正在运行
func (r *Runner) IsRunning() bool {
	if r == nil {
		return false
	}
	return r.running
}

// ==================== 同步循环 ====================

// runSyncLoop 运行同步循环
func (r *Runner) runSyncLoop(ctx context.Context, healthCancelFunc context.CancelFunc) {
	defer func() {
		// 停止健康检查
		if healthCancelFunc != nil {
			healthCancelFunc()
		}
		r.running = false
	}()

	syncInterval := r.orchestrator.GetSyncInterval()
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	// 启动时执行一次同步
	r.runSync(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("RAGFlow 同步循环收到停止信号")
			return
		case <-ticker.C:
			r.runSync(ctx)
		}
	}
}

// runSync 执行一次同步
func (r *Runner) runSync(ctx context.Context) {
	slog.Debug("开始执行定时同步...")

	if err := r.orchestrator.RunBatchSync(ctx); err != nil {
		slog.Error("批量同步执行失败", slog.Any("error", err))
	}
}

// ==================== 手动控制 ====================

// TriggerSync 手动触发一次同步
func (r *Runner) TriggerSync(ctx context.Context) error {
	if r == nil {
		return nil
	}

	slog.Info("手动触发 RAGFlow 同步...")
	return r.orchestrator.RunBatchSync(ctx)
}

// ==================== 状态查询 ====================

// GetOrchestrator 获取编排器（用于高级操作）
func (r *Runner) GetOrchestrator() *sync.Orchestrator {
	if r == nil {
		return nil
	}
	return r.orchestrator
}

// GetHealthStatus 获取健康状态
func (r *Runner) GetHealthStatus() *sync.HealthStatus {
	if r == nil {
		return nil
	}
	status := r.orchestrator.GetHealthStatus()
	return &status
}

// GetSyncStats 获取同步统计
func (r *Runner) GetSyncStats(ctx context.Context) (*sync.SyncStats, error) {
	if r == nil {
		return nil, nil
	}
	return r.orchestrator.GetSyncStats(ctx)
}

// ==================== 事件处理入口 ====================

// OnMemoCreated Memo 创建事件处理
func (r *Runner) OnMemoCreated(ctx context.Context, memo *store.Memo) {
	if r == nil || r.orchestrator == nil {
		return
	}

	// 创建同步状态记录
	if err := r.orchestrator.GetMemoSyncer().CreateSyncStateForNewMemo(ctx, memo); err != nil {
		slog.Error("创建 Memo 同步状态失败",
			slog.String("memoUID", memo.UID),
			slog.Any("error", err))
		return
	}

	// 尝试立即同步（异步）
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		event := &sync.SyncEvent{
			Type:        sync.SyncEventCreate,
			ContentType: store.ContentTypeMemo,
			ContentUID:  memo.UID,
			UserID:      memo.CreatorID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(syncCtx, event); err != nil {
			slog.Warn("Memo 同步失败，将在下次批量同步时重试",
				slog.String("memoUID", memo.UID),
				slog.Any("error", err))
		}
	}()
}

// OnMemoUpdated Memo 更新事件处理
func (r *Runner) OnMemoUpdated(ctx context.Context, memo *store.Memo) {
	if r == nil || r.orchestrator == nil {
		return
	}

	// 标记需要重新同步
	if err := r.orchestrator.GetMemoSyncer().MarkMemoForResync(ctx, memo.UID, memo.Content); err != nil {
		slog.Error("标记 Memo 重新同步失败",
			slog.String("memoUID", memo.UID),
			slog.Any("error", err))
		return
	}

	// 尝试立即同步（异步）
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		event := &sync.SyncEvent{
			Type:        sync.SyncEventUpdate,
			ContentType: store.ContentTypeMemo,
			ContentUID:  memo.UID,
			UserID:      memo.CreatorID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(syncCtx, event); err != nil {
			slog.Warn("Memo 更新同步失败，将在下次批量同步时重试",
				slog.String("memoUID", memo.UID),
				slog.Any("error", err))
		}
	}()
}

// OnMemoDeleted Memo 删除事件处理
func (r *Runner) OnMemoDeleted(ctx context.Context, memoUID string) {
	if r == nil || r.orchestrator == nil {
		return
	}

	// 同步删除（异步）
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := r.orchestrator.GetMemoSyncer().DeleteMemoFromRAGFlow(syncCtx, memoUID); err != nil {
			slog.Error("从 RAGFlow 删除 Memo 失败",
				slog.String("memoUID", memoUID),
				slog.Any("error", err))
		}
	}()
}

// OnMemoVisibilityChanged Memo visibility 变更事件处理
func (r *Runner) OnMemoVisibilityChanged(ctx context.Context, memoUID string, newVisibility store.Visibility) {
	if r == nil || r.orchestrator == nil {
		return
	}

	// 处理 visibility 变更（异步）
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := r.orchestrator.HandleVisibilityChange(syncCtx, memoUID, newVisibility); err != nil {
			slog.Warn("更新 Memo visibility 失败",
				slog.String("memoUID", memoUID),
				slog.Any("error", err))
		}
	}()
}

// OnAttachmentCreated 附件创建事件处理
func (r *Runner) OnAttachmentCreated(ctx context.Context, attachment *store.Attachment) {
	if r == nil || r.orchestrator == nil {
		return
	}

	// 创建同步状态记录
	if err := r.orchestrator.GetAttachmentSyncer().CreateSyncStateForNewAttachment(ctx, attachment); err != nil {
		slog.Error("创建附件同步状态失败",
			slog.String("attachmentUID", attachment.UID),
			slog.Any("error", err))
		return
	}

	// 检查是否为可解析类型，如果是则尝试立即同步
	if !sync.IsParseableAttachment(attachment.Type) {
		slog.Debug("附件类型不支持解析，已标记为跳过",
			slog.String("attachmentUID", attachment.UID),
			slog.String("type", attachment.Type))
		return
	}

	// 尝试立即同步（异步）
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // 附件可能较大，给更多时间
		defer cancel()

		event := &sync.SyncEvent{
			Type:        sync.SyncEventCreate,
			ContentType: store.ContentTypeAttachment,
			ContentUID:  attachment.UID,
			UserID:      attachment.CreatorID,
			Timestamp:   time.Now(),
		}

		if err := r.orchestrator.HandleSyncEvent(syncCtx, event); err != nil {
			slog.Warn("附件同步失败，将在下次批量同步时重试",
				slog.String("attachmentUID", attachment.UID),
				slog.Any("error", err))
		}
	}()
}

// OnAttachmentDeleted 附件删除事件处理
func (r *Runner) OnAttachmentDeleted(ctx context.Context, attachmentUID string) {
	if r == nil || r.orchestrator == nil {
		return
	}

	// 同步删除（异步）
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := r.orchestrator.GetAttachmentSyncer().DeleteAttachmentFromRAGFlow(syncCtx, attachmentUID); err != nil {
			slog.Error("从 RAGFlow 删除附件失败",
				slog.String("attachmentUID", attachmentUID),
				slog.Any("error", err))
		}
	}()
}

// OnUserDeleted 用户删除事件处理
func (r *Runner) OnUserDeleted(ctx context.Context, userID int32) {
	if r == nil || r.orchestrator == nil {
		return
	}

	// 清理用户的 RAGFlow 资源（异步）
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := r.orchestrator.HandleUserDeletion(syncCtx, userID); err != nil {
			slog.Error("清理用户 RAGFlow 资源失败",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
		}
	}()
}
