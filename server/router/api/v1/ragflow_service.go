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

	slog.Info("SemanticSearch 请求开始",
		slog.Int("userID", int(userID)),
		slog.String("query", req.Query))

	if req.Query == "" {
		return nil, status.Errorf(codes.InvalidArgument, "query is required")
	}

	// 获取用户信息（Provisioner 需要 username）
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		slog.Warn("SemanticSearch: 获取用户信息失败",
			slog.Int("userID", int(userID)),
			slog.Any("error", err))
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	// 通过 Provisioner 获取 per-user Client（仅认证，不创建 legacy Dataset）
	var userClient *ragflow.Client

	if s.RAGFlowProvisioner != nil {
		userClient, err = s.RAGFlowProvisioner.GetClientForUser(ctx, userID, user.Username)
		if err != nil {
			slog.Warn("SemanticSearch: 获取用户客户端失败，返回空结果",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
			return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
		}
	} else {
		// 降级：通过旧路径获取客户端
		userClient = s.getUserRAGFlowClient(ctx, userID)
		if userClient == nil {
			slog.Warn("SemanticSearch: 用户未完成 RAGFlow Provisioning，返回空结果",
				slog.Int("userID", int(userID)))
			return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
		}
	}

	// 从 notebooks 表收集所有 DatasetID，对缺失的就地补偿创建
	notebooks, err := s.Store.ListNotebooks(ctx, &store.FindNotebook{CreatorID: &userID})
	if err != nil {
		slog.Warn("SemanticSearch: 获取用户 notebooks 失败",
			slog.Int("userID", int(userID)),
			slog.Any("error", err))
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	var datasetIDs []string
	for _, nb := range notebooks {
		if nb.DatasetID != "" {
			datasetIDs = append(datasetIDs, nb.DatasetID)
		} else if s.RAGFlowProvisioner != nil {
			// 补偿：notebook 没有 dataset（创建时 RAGFlow 不可用），现在尝试创建
			dsID, err := s.createNotebookDataset(ctx, userID, user.Username, nb.Title)
			if err != nil {
				slog.Warn("SemanticSearch: 补偿创建 notebook dataset 失败",
					slog.Int("notebookID", int(nb.ID)),
					slog.Any("error", err))
				continue
			}
			updatedDsID := dsID
			if updateErr := s.Store.UpdateNotebook(ctx, &store.UpdateNotebook{ID: nb.ID, DatasetID: &updatedDsID}); updateErr != nil {
				slog.Warn("SemanticSearch: 补偿更新 notebook datasetID 失败",
					slog.Int("notebookID", int(nb.ID)),
					slog.Any("error", updateErr))
				continue
			}
			slog.Info("SemanticSearch: 补偿创建 notebook dataset 成功",
				slog.Int("notebookID", int(nb.ID)),
				slog.String("datasetID", dsID))
			datasetIDs = append(datasetIDs, dsID)
		}
	}

	if len(datasetIDs) == 0 {
		slog.Warn("SemanticSearch: 用户没有可用的 Dataset，返回空结果",
			slog.Int("userID", int(userID)))
		return &v1pb.SemanticSearchResponse{Results: []*v1pb.SearchResult{}}, nil
	}

	slog.Info("SemanticSearch: 找到用户 Datasets",
		slog.Int("userID", int(userID)),
		slog.Any("datasetIDs", datasetIDs))

	// 设置默认值
	topK := int(req.TopK)
	if topK <= 0 {
		topK = 6
	}
	similarityThreshold := req.SimilarityThreshold
	if similarityThreshold <= 0 {
		similarityThreshold = 0.3
	}

	// 使用 per-user 客户端调用 RAGFlow 检索 API
	retrieveReq := &ragflow.RetrievalRequest{
		DatasetIDs:          datasetIDs,
		Question:            req.Query,
		TopK:                topK,
		SimilarityThreshold: float64(similarityThreshold),
	}

	slog.Info("SemanticSearch: 调用 RAGFlow Retrieve API",
		slog.Any("datasetIDs", datasetIDs),
		slog.String("question", req.Query),
		slog.Int("topK", topK),
		slog.Float64("similarityThreshold", float64(similarityThreshold)))

	retrievalResult, err := userClient.Retrieve(ctx, retrieveReq)
	if err != nil {
		slog.Error("SemanticSearch: RAGFlow 检索失败",
			slog.Int("userID", int(userID)),
			slog.Any("error", err))
		return nil, status.Errorf(codes.Internal, "retrieval failed: %v", err)
	}

	// 记录检索结果
	chunkCount := 0
	if retrievalResult != nil {
		chunkCount = len(retrievalResult.Chunks)
	}
	slog.Info("SemanticSearch: RAGFlow 返回结果",
		slog.Int("userID", int(userID)),
		slog.Int("chunkCount", chunkCount))

	// 转换检索结果
	results := make([]*v1pb.SearchResult, 0)
	if retrievalResult != nil && len(retrievalResult.Chunks) > 0 {
		for _, chunk := range retrievalResult.Chunks {
			slog.Debug("SemanticSearch: 处理 chunk",
				slog.String("documentName", chunk.DocumentName),
				slog.Float64("similarity", chunk.Similarity))

			// 从文档名解析 Memo UID
			memoUID := extractMemoUIDFromDocumentName(chunk.DocumentName)
			if memoUID == "" {
				slog.Debug("SemanticSearch: 跳过非 Memo 文档",
					slog.String("documentName", chunk.DocumentName))
				continue // 跳过非 Memo 文档
			}

			// 获取 Memo 详情
			memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
			if err != nil || memo == nil {
				slog.Debug("SemanticSearch: Memo 不存在",
					slog.String("memoUID", memoUID))
				continue // 跳过不存在的 Memo
			}

			// 检查 visibility（只返回用户有权访问的内容）
			if memo.CreatorID != userID && memo.Visibility == store.Private {
				slog.Debug("SemanticSearch: 跳过私有 Memo",
					slog.String("memoUID", memoUID))
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

	slog.Info("SemanticSearch: 请求完成",
		slog.Int("userID", int(userID)),
		slog.Int("resultCount", len(results)))

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
