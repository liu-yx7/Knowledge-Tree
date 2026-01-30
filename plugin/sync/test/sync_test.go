// Package sync_test 提供 sync 模块的单元测试
package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/plugin/sync"
	"github.com/usememos/memos/store"
	storetest "github.com/usememos/memos/store/test"
)

// ==================== State Tracker 测试 ====================

func TestStateTracker_CreatePendingState(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	// 创建测试用户
	user := createTestUser(ctx, t, ts)

	// 创建 StateTracker
	tracker := sync.NewStateTracker(ts, nil)

	// 测试创建 pending 状态
	state, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_uid_001", user.ID, "hash123")
	require.NoError(t, err)
	require.NotNil(t, state)

	assert.Equal(t, store.ContentTypeMemo, state.ContentType)
	assert.Equal(t, "memo_uid_001", state.ContentUID)
	assert.Equal(t, user.ID, state.OwnerID)
	assert.Equal(t, store.RAGFlowSyncStatusPending, state.RAGFlowStatus)
	assert.Equal(t, "hash123", state.ContentHash)
	assert.Equal(t, int32(0), state.RetryCount)
}

func TestStateTracker_GetSyncState(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	tracker := sync.NewStateTracker(ts, nil)

	// 创建状态
	_, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_uid_002", user.ID, "hash456")
	require.NoError(t, err)

	// 获取状态
	state, err := tracker.GetSyncState(ctx, store.ContentTypeMemo, "memo_uid_002")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, "memo_uid_002", state.ContentUID)

	// 获取不存在的状态
	notFound, err := tracker.GetSyncState(ctx, store.ContentTypeMemo, "non_existent")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestStateTracker_MarkAsSynced(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	tracker := sync.NewStateTracker(ts, nil)

	// 创建 pending 状态
	state, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_uid_003", user.ID, "hash789")
	require.NoError(t, err)

	// 标记为已同步
	err = tracker.MarkAsSynced(ctx, state.ID, "dataset_001", "doc_001")
	require.NoError(t, err)

	// 验证状态已更新
	updatedState, err := tracker.GetSyncState(ctx, store.ContentTypeMemo, "memo_uid_003")
	require.NoError(t, err)
	assert.Equal(t, store.RAGFlowSyncStatusSynced, updatedState.RAGFlowStatus)
	assert.Equal(t, "dataset_001", updatedState.RAGFlowDatasetID)
	assert.Equal(t, "doc_001", updatedState.RAGFlowDocumentID)
	assert.NotNil(t, updatedState.RAGFlowSyncedTs)
}

func TestStateTracker_MarkAsFailed(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	cfg := &sync.StateTrackerConfig{
		MaxRetries:     3,
		BaseRetryDelay: time.Second,
	}
	tracker := sync.NewStateTracker(ts, cfg)

	// 创建 pending 状态
	state, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_uid_004", user.ID, "hash000")
	require.NoError(t, err)

	// 标记为失败
	err = tracker.MarkAsFailed(ctx, state.ID, 0, "test error message")
	require.NoError(t, err)

	// 验证状态已更新
	updatedState, err := tracker.GetSyncState(ctx, store.ContentTypeMemo, "memo_uid_004")
	require.NoError(t, err)
	assert.Equal(t, store.RAGFlowSyncStatusFailed, updatedState.RAGFlowStatus)
	assert.Equal(t, "test error message", updatedState.RAGFlowError)
	assert.Equal(t, int32(1), updatedState.RetryCount)
	assert.NotNil(t, updatedState.NextRetryTs) // 应该设置了下次重试时间
}

func TestStateTracker_MarkAsSkipped(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	tracker := sync.NewStateTracker(ts, nil)

	// 创建 pending 状态
	state, err := tracker.CreatePendingState(ctx, store.ContentTypeAttachment, "attachment_uid_001", user.ID, "")
	require.NoError(t, err)

	// 标记为跳过
	err = tracker.MarkAsSkipped(ctx, state.ID, "unsupported file type: image/gif")
	require.NoError(t, err)

	// 验证状态已更新
	updatedState, err := tracker.GetSyncState(ctx, store.ContentTypeAttachment, "attachment_uid_001")
	require.NoError(t, err)
	assert.Equal(t, store.RAGFlowSyncStatusSkipped, updatedState.RAGFlowStatus)
	assert.Equal(t, "unsupported file type: image/gif", updatedState.RAGFlowError)
}

func TestStateTracker_ListPendingStates(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	tracker := sync.NewStateTracker(ts, nil)

	// 创建多个 pending 状态
	for i := 0; i < 5; i++ {
		_, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo,
			"memo_pending_"+string(rune('a'+i)), user.ID, "hash")
		require.NoError(t, err)
	}

	// 列出 pending 状态
	pendingStates, err := tracker.ListPendingStates(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, pendingStates, 5)

	// 测试 limit
	limitedStates, err := tracker.ListPendingStates(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, limitedStates, 3)
}

func TestStateTracker_ListRetryableStates(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	cfg := &sync.StateTrackerConfig{
		MaxRetries:     3,
		BaseRetryDelay: time.Millisecond, // 使用极短的延迟以便测试
	}
	tracker := sync.NewStateTracker(ts, cfg)

	// 创建状态并标记为失败
	state, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_retry_001", user.ID, "hash")
	require.NoError(t, err)

	err = tracker.MarkAsFailed(ctx, state.ID, 0, "error 1")
	require.NoError(t, err)

	// 等待超过 retry delay
	time.Sleep(10 * time.Millisecond)

	// 获取可重试状态
	retryableStates, err := tracker.ListRetryableStates(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, retryableStates, 1)
	assert.Equal(t, "memo_retry_001", retryableStates[0].ContentUID)
}

func TestStateTracker_HasContentChanged(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	tracker := sync.NewStateTracker(ts, nil)

	// 新内容（没有同步记录）
	changed, err := tracker.HasContentChanged(ctx, store.ContentTypeMemo, "new_memo", "new_hash")
	require.NoError(t, err)
	assert.True(t, changed)

	// 创建同步记录
	_, err = tracker.CreatePendingState(ctx, store.ContentTypeMemo, "existing_memo", user.ID, "original_hash")
	require.NoError(t, err)

	// 内容未变更
	changed, err = tracker.HasContentChanged(ctx, store.ContentTypeMemo, "existing_memo", "original_hash")
	require.NoError(t, err)
	assert.False(t, changed)

	// 内容已变更
	changed, err = tracker.HasContentChanged(ctx, store.ContentTypeMemo, "existing_memo", "new_hash")
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestStateTracker_ResetForResync(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	tracker := sync.NewStateTracker(ts, nil)

	// 创建并标记为已同步的状态
	state, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_resync", user.ID, "old_hash")
	require.NoError(t, err)

	err = tracker.MarkAsSynced(ctx, state.ID, "dataset", "doc")
	require.NoError(t, err)

	// 重置以便重新同步
	err = tracker.ResetForResync(ctx, state.ID, "new_hash")
	require.NoError(t, err)

	// 验证状态已重置
	updatedState, err := tracker.GetSyncState(ctx, store.ContentTypeMemo, "memo_resync")
	require.NoError(t, err)
	assert.Equal(t, store.RAGFlowSyncStatusPending, updatedState.RAGFlowStatus)
	assert.Equal(t, "new_hash", updatedState.ContentHash)
	assert.Equal(t, int32(0), updatedState.RetryCount)
}

func TestStateTracker_DeleteState(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user := createTestUser(ctx, t, ts)
	tracker := sync.NewStateTracker(ts, nil)

	// 创建状态
	_, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_delete", user.ID, "hash")
	require.NoError(t, err)

	// 删除状态
	err = tracker.DeleteState(ctx, store.ContentTypeMemo, "memo_delete")
	require.NoError(t, err)

	// 验证已删除
	state, err := tracker.GetSyncState(ctx, store.ContentTypeMemo, "memo_delete")
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestStateTracker_DeleteStatesByOwner(t *testing.T) {
	ctx := context.Background()
	ts := storetest.NewTestingStore(ctx, t)

	user1 := createTestUser(ctx, t, ts)
	user2 := createTestUserWithUsername(ctx, t, ts, "user2")
	tracker := sync.NewStateTracker(ts, nil)

	// 为 user1 创建状态
	_, err := tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_u1_1", user1.ID, "hash")
	require.NoError(t, err)
	_, err = tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_u1_2", user1.ID, "hash")
	require.NoError(t, err)

	// 为 user2 创建状态
	_, err = tracker.CreatePendingState(ctx, store.ContentTypeMemo, "memo_u2_1", user2.ID, "hash")
	require.NoError(t, err)

	// 删除 user1 的所有状态
	err = tracker.DeleteStatesByOwner(ctx, user1.ID)
	require.NoError(t, err)

	// 验证 user1 的状态已删除
	states, err := tracker.ListStatesByOwner(ctx, user1.ID)
	require.NoError(t, err)
	assert.Len(t, states, 0)

	// 验证 user2 的状态仍存在
	states, err = tracker.ListStatesByOwner(ctx, user2.ID)
	require.NoError(t, err)
	assert.Len(t, states, 1)
}

// ==================== ComputeContentHash 测试 ====================

func TestComputeContentHash(t *testing.T) {
	// 测试哈希计算
	hash1 := sync.ComputeContentHash("Hello, World!")
	hash2 := sync.ComputeContentHash("Hello, World!")
	hash3 := sync.ComputeContentHash("Different content")

	// 相同内容应产生相同哈希
	assert.Equal(t, hash1, hash2)

	// 不同内容应产生不同哈希
	assert.NotEqual(t, hash1, hash3)

	// 哈希应为有效的 hex 字符串
	assert.Len(t, hash1, 64) // SHA-256 produces 64 hex characters

	// 空字符串也应该能计算
	emptyHash := sync.ComputeContentHash("")
	assert.Len(t, emptyHash, 64)
}

// ==================== Health Checker 测试 ====================

func TestHealthChecker_DefaultConfig(t *testing.T) {
	cfg := sync.DefaultHealthCheckerConfig()
	assert.Equal(t, 30*time.Second, cfg.CheckInterval)
	assert.Equal(t, 3, cfg.FailureThreshold)
	assert.Equal(t, 60*time.Second, cfg.RecoveryTimeout)
}

func TestHealthChecker_GetStatus(t *testing.T) {
	// 注意：此测试不需要真实的 RAGFlow 客户端
	// 因为 GetStatus 只返回当前状态，不执行实际检查
	cfg := sync.DefaultHealthCheckerConfig()

	// 创建一个 nil client 的 HealthChecker（仅用于测试状态）
	checker := sync.NewHealthChecker(nil, cfg)

	status := checker.GetStatus()
	assert.True(t, status.IsHealthy) // 初始状态为健康
	assert.False(t, status.CircuitOpen)
	assert.Equal(t, 0, status.FailureCount)
}

func TestHealthChecker_RecordSuccessAndFailure(t *testing.T) {
	cfg := &sync.HealthCheckerConfig{
		CheckInterval:    time.Second,
		FailureThreshold: 2,
		RecoveryTimeout:  time.Second,
	}
	checker := sync.NewHealthChecker(nil, cfg)

	// 记录失败
	checker.RecordFailure(nil)
	status := checker.GetStatus()
	assert.Equal(t, 1, status.FailureCount)
	assert.False(t, status.CircuitOpen) // 未达到阈值

	// 再次失败，触发熔断
	checker.RecordFailure(nil)
	status = checker.GetStatus()
	assert.Equal(t, 2, status.FailureCount)
	assert.True(t, status.CircuitOpen) // 达到阈值，熔断打开

	// 记录成功，恢复
	checker.RecordSuccess()
	status = checker.GetStatus()
	assert.Equal(t, 0, status.FailureCount)
	assert.False(t, status.CircuitOpen)
}

func TestHealthChecker_ForceOpenClose(t *testing.T) {
	checker := sync.NewHealthChecker(nil, nil)

	// 强制打开熔断
	checker.ForceOpen()
	status := checker.GetStatus()
	assert.True(t, status.CircuitOpen)
	assert.False(t, status.IsHealthy)

	// 强制关闭熔断
	checker.ForceClose()
	status = checker.GetStatus()
	assert.False(t, status.CircuitOpen)
	assert.True(t, status.IsHealthy)
}

// ==================== Attachment Syncer 附件类型判断测试 ====================

func TestIsParseableAttachment(t *testing.T) {
	tests := []struct {
		mimeType string
		expected bool
	}{
		// 可解析类型
		{"application/pdf", true},
		{"text/plain", true},
		{"text/markdown", true},
		{"text/html", true},
		{"application/msword", true},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"application/vnd.ms-excel", true},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"application/vnd.ms-powerpoint", true},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", true},
		{"image/png", true},
		{"image/jpeg", true},
		{"image/jpg", true},
		{"image/tiff", true},
		{"image/bmp", true},
		{"image/webp", true},

		// 不可解析类型
		{"image/gif", false},
		{"video/mp4", false},
		{"audio/mp3", false},
		{"application/zip", false},
		{"application/json", false},
		{"unknown/type", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			result := sync.IsParseableAttachment(tt.mimeType)
			assert.Equal(t, tt.expected, result, "mimeType: %s", tt.mimeType)
		})
	}
}

// ==================== Orchestrator Config 测试 ====================

func TestOrchestratorConfig_Defaults(t *testing.T) {
	cfg := sync.DefaultOrchestratorConfig()
	assert.Equal(t, 50, cfg.BatchSize)
	assert.Equal(t, 5*time.Minute, cfg.SyncInterval)
	assert.NotNil(t, cfg.StateTrackerConfig)
	assert.NotNil(t, cfg.HealthCheckerConfig)
}

// ==================== SyncEvent 测试 ====================

func TestSyncEvent_Types(t *testing.T) {
	assert.Equal(t, sync.SyncEventType("create"), sync.SyncEventCreate)
	assert.Equal(t, sync.SyncEventType("update"), sync.SyncEventUpdate)
	assert.Equal(t, sync.SyncEventType("delete"), sync.SyncEventDelete)
}

// ==================== 辅助函数 ====================

func createTestUser(ctx context.Context, t *testing.T, s *store.Store) *store.User {
	return createTestUserWithUsername(ctx, t, s, "test_user")
}

func createTestUserWithUsername(ctx context.Context, t *testing.T, s *store.Store, username string) *store.User {
	user, err := s.CreateUser(ctx, &store.User{
		Username: username,
		Role:     store.RoleAdmin,
		Email:    username + "@test.com",
		Nickname: "Test User",
	})
	require.NoError(t, err)
	return user
}
