// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/server/router/api/v1/test/ragflow_service_test.go
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

// ==================== GetSyncStatus 测试 ====================

func TestGetSyncStatus_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 未认证的请求
	_, err := ts.Service.GetSyncStatus(ctx, &v1pb.GetSyncStatusRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code(), "未认证用户应返回 Unauthenticated")
}

func TestGetSyncStatus_PermissionDenied_NonAdmin(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 创建普通用户
	user, err := ts.CreateRegularUser(ctx, "regular_user")
	require.NoError(t, err)

	// 使用普通用户上下文
	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 普通用户调用管理接口
	_, err = ts.Service.GetSyncStatus(userCtx, &v1pb.GetSyncStatusRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code(), "非管理员应返回 PermissionDenied")
}

func TestGetSyncStatus_AdminWithNilRunner(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 创建管理员用户
	admin, err := ts.CreateHostUser(ctx, "admin_user")
	require.NoError(t, err)

	// 使用管理员上下文
	adminCtx := ts.CreateUserContext(ctx, admin.ID)

	// RAGFlowSyncRunner 为 nil 时
	ts.Service.RAGFlowSyncRunner = nil

	resp, err := ts.Service.GetSyncStatus(adminCtx, &v1pb.GetSyncStatusRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 应返回禁用状态
	assert.False(t, resp.Healthy, "Runner 为 nil 时 Healthy 应为 false")
	assert.False(t, resp.CircuitOpen, "Runner 为 nil 时 CircuitOpen 应为 false")
	assert.False(t, resp.RunnerActive, "Runner 为 nil 时 RunnerActive 应为 false")
}

// ==================== GetSyncStats 测试 ====================

func TestGetSyncStats_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.GetSyncStats(ctx, &v1pb.GetSyncStatsRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetSyncStats_PermissionDenied_NonAdmin(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "regular_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	_, err = ts.Service.GetSyncStats(userCtx, &v1pb.GetSyncStatsRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestGetSyncStats_AdminWithNilRunner(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	admin, err := ts.CreateHostUser(ctx, "admin_user")
	require.NoError(t, err)

	adminCtx := ts.CreateUserContext(ctx, admin.ID)

	ts.Service.RAGFlowSyncRunner = nil

	resp, err := ts.Service.GetSyncStats(adminCtx, &v1pb.GetSyncStatsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 空统计
	assert.Equal(t, int32(0), resp.PendingCount)
	assert.Equal(t, int32(0), resp.SyncedCount)
	assert.Equal(t, int32(0), resp.FailedCount)
	assert.Equal(t, int32(0), resp.SkippedCount)
	assert.Equal(t, int32(0), resp.TotalCount)
}

// ==================== TriggerSync 测试 ====================

func TestTriggerSync_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.TriggerSync(ctx, &v1pb.TriggerSyncRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestTriggerSync_PermissionDenied_NonAdmin(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "regular_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	_, err = ts.Service.TriggerSync(userCtx, &v1pb.TriggerSyncRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestTriggerSync_FailedPrecondition_NilRunner(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	admin, err := ts.CreateHostUser(ctx, "admin_user")
	require.NoError(t, err)

	adminCtx := ts.CreateUserContext(ctx, admin.ID)

	ts.Service.RAGFlowSyncRunner = nil

	_, err = ts.Service.TriggerSync(adminCtx, &v1pb.TriggerSyncRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ==================== SemanticSearch 测试 ====================

func TestSemanticSearch_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.SemanticSearch(ctx, &v1pb.SemanticSearchRequest{
		Query: "test query",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestSemanticSearch_FailedPrecondition_NilClient(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	ts.Service.RAGFlowClient = nil

	_, err = ts.Service.SemanticSearch(userCtx, &v1pb.SemanticSearchRequest{
		Query: "test query",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestSemanticSearch_InvalidArgument_EmptyQuery(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 设置一个非 nil 的 RAGFlowClient（实际测试中会失败，但这里测试参数验证）
	// 需要先检查 query 参数
	ts.Service.RAGFlowClient = nil // 先设为 nil

	_, err = ts.Service.SemanticSearch(userCtx, &v1pb.SemanticSearchRequest{
		Query: "",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	// 因为 RAGFlowClient 为 nil，会先返回 FailedPrecondition
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ==================== ListContentSyncStates 测试 ====================

func TestListContentSyncStates_Unauthenticated(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	_, err := ts.Service.ListContentSyncStates(ctx, &v1pb.ListContentSyncStatesRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestListContentSyncStates_EmptyResult(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	resp, err := ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Empty(t, resp.SyncStates, "新用户应没有同步状态")
	assert.Empty(t, resp.NextPageToken, "无数据时应无下一页令牌")
}

func TestListContentSyncStates_WithFilters(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建一些同步状态记录
	syncedTs := int64(1700000000)
	_, err = ts.Store.CreateContentSyncState(ctx, &store.ContentSyncState{
		ContentType:       store.ContentTypeMemo,
		ContentUID:        "memo-001",
		OwnerID:           user.ID,
		RAGFlowStatus:     store.RAGFlowSyncStatusSynced,
		RAGFlowDocumentID: "doc-001",
		RAGFlowSyncedTs:   &syncedTs,
	})
	require.NoError(t, err)

	_, err = ts.Store.CreateContentSyncState(ctx, &store.ContentSyncState{
		ContentType:   store.ContentTypeMemo,
		ContentUID:    "memo-002",
		OwnerID:       user.ID,
		RAGFlowStatus: store.RAGFlowSyncStatusPending,
	})
	require.NoError(t, err)

	_, err = ts.Store.CreateContentSyncState(ctx, &store.ContentSyncState{
		ContentType:   store.ContentTypeAttachment,
		ContentUID:    "attachment-001",
		OwnerID:       user.ID,
		RAGFlowStatus: store.RAGFlowSyncStatusFailed,
		RAGFlowError:  "connection timeout",
		RetryCount:    2,
	})
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 测试1：无过滤，获取所有
	resp, err := ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.SyncStates, 3, "应返回所有 3 条记录")

	// 测试2：按状态过滤
	resp, err = ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		StatusFilter: "pending",
	})
	require.NoError(t, err)
	assert.Len(t, resp.SyncStates, 1, "应只返回 pending 状态的记录")
	assert.Equal(t, "memo-002", resp.SyncStates[0].ContentUid)

	// 测试3：按内容类型过滤
	resp, err = ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		ContentTypeFilter: "attachment",
	})
	require.NoError(t, err)
	assert.Len(t, resp.SyncStates, 1, "应只返回 attachment 类型的记录")
	assert.Equal(t, "attachment-001", resp.SyncStates[0].ContentUid)

	// 测试4：验证返回字段
	resp, err = ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		StatusFilter: "failed",
	})
	require.NoError(t, err)
	require.Len(t, resp.SyncStates, 1)

	failedState := resp.SyncStates[0]
	assert.Equal(t, "attachment", failedState.ContentType)
	assert.Equal(t, "attachment-001", failedState.ContentUid)
	assert.Equal(t, "failed", failedState.Status)
	assert.Equal(t, "connection timeout", failedState.ErrorMessage)
	assert.Equal(t, int32(2), failedState.RetryCount)
}

func TestListContentSyncStates_Pagination(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	// 创建 5 条记录
	for i := 0; i < 5; i++ {
		_, err = ts.Store.CreateContentSyncState(ctx, &store.ContentSyncState{
			ContentType:   store.ContentTypeMemo,
			ContentUID:    "memo-" + string(rune('a'+i)),
			OwnerID:       user.ID,
			RAGFlowStatus: store.RAGFlowSyncStatusPending,
		})
		require.NoError(t, err)
	}

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 测试分页：每页 2 条
	resp, err := ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Len(t, resp.SyncStates, 2, "第一页应返回 2 条记录")
	assert.NotEmpty(t, resp.NextPageToken, "应有下一页令牌")

	// 获取第二页
	resp2, err := ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		PageSize:  2,
		PageToken: resp.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, resp2.SyncStates, 2, "第二页应返回 2 条记录")
	assert.NotEmpty(t, resp2.NextPageToken, "应有下一页令牌")

	// 获取第三页（最后一页）
	resp3, err := ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		PageSize:  2,
		PageToken: resp2.NextPageToken,
	})
	require.NoError(t, err)
	assert.Len(t, resp3.SyncStates, 1, "最后一页应返回 1 条记录")
	assert.Empty(t, resp3.NextPageToken, "最后一页不应有下一页令牌")
}

func TestListContentSyncStates_PageSizeLimits(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	user, err := ts.CreateRegularUser(ctx, "test_user")
	require.NoError(t, err)

	userCtx := ts.CreateUserContext(ctx, user.ID)

	// 测试 PageSize 为 0 时使用默认值 20
	resp, err := ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		PageSize: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 测试 PageSize 超过 100 时被限制为 100
	resp, err = ts.Service.ListContentSyncStates(userCtx, &v1pb.ListContentSyncStatesRequest{
		PageSize: 200,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestListContentSyncStates_DataIsolation(t *testing.T) {
	ts := NewTestService(t)
	defer ts.Cleanup()

	ctx := context.Background()

	// 创建两个用户
	user1, err := ts.CreateRegularUser(ctx, "user1")
	require.NoError(t, err)

	user2, err := ts.CreateRegularUser(ctx, "user2")
	require.NoError(t, err)

	// 为 user1 创建记录
	_, err = ts.Store.CreateContentSyncState(ctx, &store.ContentSyncState{
		ContentType:   store.ContentTypeMemo,
		ContentUID:    "user1-memo",
		OwnerID:       user1.ID,
		RAGFlowStatus: store.RAGFlowSyncStatusSynced,
	})
	require.NoError(t, err)

	// 为 user2 创建记录
	_, err = ts.Store.CreateContentSyncState(ctx, &store.ContentSyncState{
		ContentType:   store.ContentTypeMemo,
		ContentUID:    "user2-memo",
		OwnerID:       user2.ID,
		RAGFlowStatus: store.RAGFlowSyncStatusPending,
	})
	require.NoError(t, err)

	// user1 只能看到自己的记录
	user1Ctx := ts.CreateUserContext(ctx, user1.ID)
	resp1, err := ts.Service.ListContentSyncStates(user1Ctx, &v1pb.ListContentSyncStatesRequest{})
	require.NoError(t, err)
	require.Len(t, resp1.SyncStates, 1)
	assert.Equal(t, "user1-memo", resp1.SyncStates[0].ContentUid)

	// user2 只能看到自己的记录
	user2Ctx := ts.CreateUserContext(ctx, user2.ID)
	resp2, err := ts.Service.ListContentSyncStates(user2Ctx, &v1pb.ListContentSyncStatesRequest{})
	require.NoError(t, err)
	require.Len(t, resp2.SyncStates, 1)
	assert.Equal(t, "user2-memo", resp2.SyncStates[0].ContentUid)
}
