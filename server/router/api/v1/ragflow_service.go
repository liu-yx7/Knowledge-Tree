package v1

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/plugin/ragflow"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

// ==================== RAGFlow Service 实现 ====================

// GetSyncStatus 获取 RAGFlow 同步服务状态
func (s *APIV1Service) GetSyncStatus(ctx context.Context, _ *v1pb.GetSyncStatusRequest) (*v1pb.SyncStatus, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 检查是否为管理员
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}
	if user.Role != store.RoleAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin permission required")
	}

	// 如果 Runner 未初始化，返回禁用状态
	if s.RAGFlowSyncRunner == nil {
		return &v1pb.SyncStatus{
			Healthy:      false,
			CircuitOpen:  false,
			RunnerActive: false,
		}, nil
	}

	healthStatus := s.RAGFlowSyncRunner.GetHealthStatus()
	if healthStatus == nil {
		return &v1pb.SyncStatus{
			Healthy:      false,
			CircuitOpen:  false,
			RunnerActive: s.RAGFlowSyncRunner.IsRunning(),
		}, nil
	}

	return &v1pb.SyncStatus{
		Healthy:       healthStatus.IsHealthy,
		CircuitOpen:   healthStatus.CircuitOpen,
		LastCheckTime: timestamppb.New(healthStatus.LastCheckTime),
		FailureCount:  int32(healthStatus.FailureCount),
		RunnerActive:  s.RAGFlowSyncRunner.IsRunning(),
	}, nil
}

// GetSyncStats 获取同步统计信息
func (s *APIV1Service) GetSyncStats(ctx context.Context, _ *v1pb.GetSyncStatsRequest) (*v1pb.SyncStats, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 检查是否为管理员
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}
	if user.Role != store.RoleAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin permission required")
	}

	if s.RAGFlowSyncRunner == nil {
		return &v1pb.SyncStats{}, nil
	}

	stats, err := s.RAGFlowSyncRunner.GetSyncStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get sync stats: %v", err)
	}
	if stats == nil {
		return &v1pb.SyncStats{}, nil
	}

	return &v1pb.SyncStats{
		PendingCount: int32(stats.PendingCount),
		SyncedCount:  int32(stats.SyncedCount),
		FailedCount:  int32(stats.FailedCount),
		SkippedCount: int32(stats.SkippedCount),
		TotalCount:   int32(stats.TotalCount),
	}, nil
}

// TriggerSync 手动触发一次同步
func (s *APIV1Service) TriggerSync(ctx context.Context, _ *v1pb.TriggerSyncRequest) (*emptypb.Empty, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 检查是否为管理员
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}
	if user.Role != store.RoleAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin permission required")
	}

	if s.RAGFlowSyncRunner == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "RAGFlow sync is not enabled")
	}

	// 异步触发同步
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.RAGFlowSyncRunner.TriggerSync(syncCtx); err != nil {
			slog.Error("手动触发同步失败", slog.Any("error", err))
		}
	}()

	return &emptypb.Empty{}, nil
}

// SemanticSearch 执行语义检索
func (s *APIV1Service) SemanticSearch(ctx context.Context, req *v1pb.SemanticSearchRequest) (*v1pb.SemanticSearchResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 检查 RAGFlow 服务是否已启用
	if s.RAGFlowClient == nil {
		slog.Debug("RAGFlow client not configured, returning empty results")
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	if req.Query == "" {
		return nil, status.Errorf(codes.InvalidArgument, "query is required")
	}

	// 检查 RAGFlow Sync Runner 是否已启用
	if s.RAGFlowSyncRunner == nil {
		slog.Debug("RAGFlow sync runner not configured, returning empty results")
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	orchestrator := s.RAGFlowSyncRunner.GetOrchestrator()
	if orchestrator == nil {
		slog.Debug("RAGFlow orchestrator not available, returning empty results")
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	// 获取用户的 Dataset ID
	datasetID, err := orchestrator.GetUserDatasetID(ctx, userID)
	if err != nil {
		// 用户可能还没有任何内容同步，返回空结果
		slog.Debug("用户没有 RAGFlow Dataset", slog.Int("userID", int(userID)), slog.Any("error", err))
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	if datasetID == "" {
		slog.Debug("用户的 Dataset ID 为空", slog.Int("userID", int(userID)))
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	// 设置默认值
	topK := int(req.TopK)
	if topK <= 0 {
		topK = 6
	}
	similarityThreshold := req.SimilarityThreshold
	if similarityThreshold <= 0 {
		similarityThreshold = 0.3
	}

	// 调用 RAGFlow 检索 API
	retrieveReq := &ragflow.RetrievalRequest{
		DatasetIDs:          []string{datasetID},
		Question:            req.Query,
		TopK:                topK,
		SimilarityThreshold: float64(similarityThreshold),
	}

	retrievalResult, err := s.RAGFlowClient.Retrieve(ctx, retrieveReq)
	if err != nil {
		slog.Error("RAGFlow 检索失败", slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "retrieval failed: %v", err)
	}

	// 转换检索结果
	results := make([]*v1pb.SearchResult, 0)
	if retrievalResult != nil && len(retrievalResult.Chunks) > 0 {
		for _, chunk := range retrievalResult.Chunks {
			// 从文档名解析 Memo UID
			memoUID := extractMemoUIDFromDocumentName(chunk.DocumentName)
			if memoUID == "" {
				continue // 跳过非 Memo 文档
			}

			// 获取 Memo 详情
			memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
			if err != nil || memo == nil {
				continue // 跳过不存在的 Memo
			}

			// 检查 visibility（只返回用户有权访问的内容）
			if memo.CreatorID != userID && memo.Visibility == store.Private {
				continue // 跳过其他用户的私有内容
			}

			result := &v1pb.SearchResult{
				MemoName:        fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
				ContentSnippet:  chunk.Content,
				SimilarityScore: float32(chunk.Similarity),
				Title:           extractMemoTitle(memo.Content),
				CreateTime:      timestamppb.New(time.Unix(memo.CreatedTs, 0)),
			}
			results = append(results, result)
		}
	}

	return &v1pb.SemanticSearchResponse{Results: results}, nil
}

// ListContentSyncStates 列出内容同步状态
func (s *APIV1Service) ListContentSyncStates(ctx context.Context, req *v1pb.ListContentSyncStatesRequest) (*v1pb.ListContentSyncStatesResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 构建查询条件
	find := &store.FindContentSyncState{
		OwnerID: &userID,
	}

	// 状态过滤
	if req.StatusFilter != "" {
		status := store.RAGFlowSyncStatus(req.StatusFilter)
		find.RAGFlowStatus = &status
	}

	// 内容类型过滤
	if req.ContentTypeFilter != "" {
		contentType := store.ContentType(req.ContentTypeFilter)
		find.ContentType = &contentType
	}

	// 分页
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	find.Limit = &pageSize

	offset := 0
	if req.PageToken != "" {
		fmt.Sscanf(req.PageToken, "%d", &offset)
	}
	find.Offset = &offset

	states, err := s.Store.ListContentSyncStates(ctx, find)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list sync states: %v", err)
	}

	syncStates := make([]*v1pb.ContentSyncState, 0, len(states))
	for _, state := range states {
		syncState := &v1pb.ContentSyncState{
			ContentType:       string(state.ContentType),
			ContentUid:        state.ContentUID,
			Status:            string(state.RAGFlowStatus),
			RagflowDocumentId: state.RAGFlowDocumentID,
			ErrorMessage:      state.RAGFlowError,
			RetryCount:        state.RetryCount,
		}
		if state.RAGFlowSyncedTs != nil && *state.RAGFlowSyncedTs > 0 {
			syncState.SyncedTime = timestamppb.New(time.Unix(*state.RAGFlowSyncedTs, 0))
		}
		syncStates = append(syncStates, syncState)
	}

	// 生成下一页令牌
	nextPageToken := ""
	if len(states) == pageSize {
		nextPageToken = fmt.Sprintf("%d", offset+pageSize)
	}

	return &v1pb.ListContentSyncStatesResponse{
		SyncStates:    syncStates,
		NextPageToken: nextPageToken,
	}, nil
}

// ==================== 辅助函数 ====================

// extractMemoUIDFromDocumentName 从 RAGFlow 文档名提取 Memo UID
// 文档名格式: memo_{uid}.md
func extractMemoUIDFromDocumentName(docName string) string {
	if !strings.HasPrefix(docName, "memo_") {
		return ""
	}
	// 去掉 "memo_" 前缀和 ".md" 后缀
	uid := strings.TrimPrefix(docName, "memo_")
	uid = strings.TrimSuffix(uid, ".md")
	return uid
}

// extractMemoTitle 从 Memo 内容提取标题（第一行或前50字符）
func extractMemoTitle(content string) string {
	// 获取第一行
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return ""
	}

	title := strings.TrimSpace(lines[0])

	// 去掉 Markdown 标题符号
	title = strings.TrimLeft(title, "# ")

	// 限制长度
	if len(title) > 50 {
		title = title[:50] + "..."
	}

	if title == "" && len(content) > 0 {
		// 如果第一行为空，取前50字符
		if len(content) > 50 {
			title = content[:50] + "..."
		} else {
			title = content
		}
	}

	return title
}
