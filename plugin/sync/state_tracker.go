// Package sync 提供 RAGFlow 同步编排功能
// 职责：协调 Memos 数据与 RAGFlow 之间的同步
package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/pkg/errors"

	"github.com/usememos/memos/store"
)

// ==================== State Tracker 定义 ====================

// StateTracker 同步状态追踪器
// 职责：追踪内容的同步状态，管理重试策略
type StateTracker struct {
	store *store.Store

	// 重试配置
	maxRetries     int           // 最大重试次数
	baseRetryDelay time.Duration // 基础重试延迟（用于指数退避）
}

// StateTrackerConfig 状态追踪器配置
type StateTrackerConfig struct {
	MaxRetries     int           // 默认 3
	BaseRetryDelay time.Duration // 默认 1 分钟
}

// DefaultStateTrackerConfig 返回默认配置
func DefaultStateTrackerConfig() *StateTrackerConfig {
	return &StateTrackerConfig{
		MaxRetries:     3,
		BaseRetryDelay: time.Minute,
	}
}

// NewStateTracker 创建状态追踪器
func NewStateTracker(s *store.Store, cfg *StateTrackerConfig) *StateTracker {
	if cfg == nil {
		cfg = DefaultStateTrackerConfig()
	}
	return &StateTracker{
		store:          s,
		maxRetries:     cfg.MaxRetries,
		baseRetryDelay: cfg.BaseRetryDelay,
	}
}

// ==================== 状态管理方法 ====================

// GetSyncState 获取内容的同步状态
func (t *StateTracker) GetSyncState(ctx context.Context, contentType store.ContentType, contentUID string) (*store.ContentSyncState, error) {
	return t.store.GetContentSyncState(ctx, &store.FindContentSyncState{
		ContentType: &contentType,
		ContentUID:  &contentUID,
	})
}

// CreatePendingState 创建待同步状态记录
func (t *StateTracker) CreatePendingState(ctx context.Context, contentType store.ContentType, contentUID string, ownerID int32, contentHash string) (*store.ContentSyncState, error) {
	state := &store.ContentSyncState{
		ContentType:   contentType,
		ContentUID:    contentUID,
		OwnerID:       ownerID,
		RAGFlowStatus: store.RAGFlowSyncStatusPending,
		ContentHash:   contentHash,
		RetryCount:    0,
	}
	return t.store.UpsertContentSyncState(ctx, state)
}

// EnsurePendingState 确保内容有同步状态记录，如果不存在则创建 pending 状态
// 与 CreatePendingState 不同，此方法是非破坏性的：已存在的记录不会被覆盖。
// 适用于：事件入口持久化、catch-up 扫描补齐历史内容。
func (t *StateTracker) EnsurePendingState(ctx context.Context, contentType store.ContentType, contentUID string, ownerID int32) error {
	existing, err := t.GetSyncState(ctx, contentType, contentUID)
	if err != nil {
		return errors.Wrap(err, "检查同步状态失败")
	}
	if existing != nil {
		// 已有记录，不覆盖
		return nil
	}
	// 不存在，用安全的 INSERT 创建
	_, err = t.store.CreateContentSyncState(ctx, &store.ContentSyncState{
		ContentType:   contentType,
		ContentUID:    contentUID,
		OwnerID:       ownerID,
		RAGFlowStatus: store.RAGFlowSyncStatusPending,
		ContentHash:   "",
		RetryCount:    0,
	})
	return err
}

// MarkAsSynced 标记为已同步
func (t *StateTracker) MarkAsSynced(ctx context.Context, stateID int32, datasetID, documentID string) error {
	now := time.Now().Unix()
	status := store.RAGFlowSyncStatusSynced
	emptyError := ""

	return t.store.UpdateContentSyncState(ctx, &store.UpdateContentSyncState{
		ID:                stateID,
		RAGFlowStatus:     &status,
		RAGFlowDatasetID:  &datasetID,
		RAGFlowDocumentID: &documentID,
		RAGFlowSyncedTs:   &now,
		RAGFlowError:      &emptyError,
		UpdatedTs:         &now,
	})
}

// MarkAsFailed 标记为同步失败，计算下次重试时间
func (t *StateTracker) MarkAsFailed(ctx context.Context, stateID int32, retryCount int32, errMsg string) error {
	now := time.Now().Unix()
	status := store.RAGFlowSyncStatusFailed
	newRetryCount := retryCount + 1

	// 计算下次重试时间（指数退避）
	var nextRetryTs *int64
	if int(newRetryCount) < t.maxRetries {
		// 指数退避: baseDelay * 2^retryCount
		delay := t.baseRetryDelay * time.Duration(1<<newRetryCount)
		nextRetry := time.Now().Add(delay).Unix()
		nextRetryTs = &nextRetry
	}
	// 如果超过最大重试次数，不设置下次重试时间

	return t.store.UpdateContentSyncState(ctx, &store.UpdateContentSyncState{
		ID:            stateID,
		RAGFlowStatus: &status,
		RAGFlowError:  &errMsg,
		RetryCount:    &newRetryCount,
		NextRetryTs:   nextRetryTs,
		UpdatedTs:     &now,
	})
}

// MarkAsSkipped 标记为跳过（不可解析的内容）
func (t *StateTracker) MarkAsSkipped(ctx context.Context, stateID int32, reason string) error {
	now := time.Now().Unix()
	status := store.RAGFlowSyncStatusSkipped

	return t.store.UpdateContentSyncState(ctx, &store.UpdateContentSyncState{
		ID:            stateID,
		RAGFlowStatus: &status,
		RAGFlowError:  &reason,
		UpdatedTs:     &now,
	})
}

// ==================== 批量查询方法 ====================

// ListPendingStates 获取待同步的状态列表
func (t *StateTracker) ListPendingStates(ctx context.Context, limit int) ([]*store.ContentSyncState, error) {
	status := store.RAGFlowSyncStatusPending
	return t.store.ListContentSyncStates(ctx, &store.FindContentSyncState{
		RAGFlowStatus: &status,
		Limit:         &limit,
	})
}

// ListRetryableStates 获取可重试的失败状态列表
// 条件：status = failed AND retry_count < maxRetries AND next_retry_ts <= now
func (t *StateTracker) ListRetryableStates(ctx context.Context, limit int) ([]*store.ContentSyncState, error) {
	status := store.RAGFlowSyncStatusFailed
	states, err := t.store.ListContentSyncStates(ctx, &store.FindContentSyncState{
		RAGFlowStatus: &status,
		Limit:         &limit,
	})
	if err != nil {
		return nil, err
	}

	// 过滤出可重试的状态
	now := time.Now().Unix()
	retryable := make([]*store.ContentSyncState, 0)
	for _, state := range states {
		if int(state.RetryCount) < t.maxRetries &&
			state.NextRetryTs != nil &&
			*state.NextRetryTs <= now {
			retryable = append(retryable, state)
		}
	}

	return retryable, nil
}

// ListStatesByOwner 获取用户的所有同步状态
func (t *StateTracker) ListStatesByOwner(ctx context.Context, ownerID int32) ([]*store.ContentSyncState, error) {
	return t.store.ListContentSyncStates(ctx, &store.FindContentSyncState{
		OwnerID: &ownerID,
	})
}

// ==================== 工具方法 ====================

// ComputeContentHash 计算内容的 SHA-256 哈希值
func ComputeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// HasContentChanged 检查内容是否发生变更
func (t *StateTracker) HasContentChanged(ctx context.Context, contentType store.ContentType, contentUID string, newHash string) (bool, error) {
	state, err := t.GetSyncState(ctx, contentType, contentUID)
	if err != nil {
		return false, errors.Wrap(err, "获取同步状态失败")
	}

	// 如果没有同步记录，认为是新内容
	if state == nil {
		return true, nil
	}

	// 比较哈希值
	return state.ContentHash != newHash, nil
}

// ResetForResync 重置状态以便重新同步
func (t *StateTracker) ResetForResync(ctx context.Context, stateID int32, newHash string) error {
	now := time.Now().Unix()
	status := store.RAGFlowSyncStatusPending
	retryCount := int32(0)

	return t.store.UpdateContentSyncState(ctx, &store.UpdateContentSyncState{
		ID:            stateID,
		RAGFlowStatus: &status,
		ContentHash:   &newHash,
		RetryCount:    &retryCount,
		UpdatedTs:     &now,
	})
}

// DeleteState 删除同步状态
func (t *StateTracker) DeleteState(ctx context.Context, contentType store.ContentType, contentUID string) error {
	return t.store.DeleteContentSyncState(ctx, &store.DeleteContentSyncState{
		ContentType: &contentType,
		ContentUID:  &contentUID,
	})
}

// DeleteStatesByOwner 删除用户的所有同步状态
func (t *StateTracker) DeleteStatesByOwner(ctx context.Context, ownerID int32) error {
	return t.store.DeleteContentSyncState(ctx, &store.DeleteContentSyncState{
		OwnerID: &ownerID,
	})
}
