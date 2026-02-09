package sync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pkg/errors"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// ==================== Memo Syncer 定义 ====================

// MemoSyncer Memo 同步器
// 职责：将 Memo 内容同步到 RAGFlow Dataset
type MemoSyncer struct {
	store         *store.Store
	ragflowClient *ragflow.Client
	stateTracker  *StateTracker
}

// NewMemoSyncer 创建 Memo 同步器
func NewMemoSyncer(s *store.Store, client *ragflow.Client, tracker *StateTracker) *MemoSyncer {
	return &MemoSyncer{
		store:         s,
		ragflowClient: client,
		stateTracker:  tracker,
	}
}

// SetClient 替换当前使用的 RAGFlow 客户端（用于注入 per-user Client）
func (s *MemoSyncer) SetClient(client *ragflow.Client) {
	s.ragflowClient = client
}

// ==================== 同步方法 ====================

// SyncMemo 同步单个 Memo 到 RAGFlow
// 流程：
// 1. 获取 Memo 内容
// 2. 计算内容哈希，检查是否需要同步
// 3. 获取或创建用户 Dataset
// 4. 上传文档到 RAGFlow
// 5. 触发解析
// 6. 更新同步状态
func (s *MemoSyncer) SyncMemo(ctx context.Context, memoUID string, datasetID string) error {
	// 1. 获取 Memo
	memo, err := s.store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return errors.Wrap(err, "获取 Memo 失败")
	}
	if memo == nil {
		return errors.New("Memo 不存在")
	}

	// 2. 计算内容哈希
	contentHash := ComputeContentHash(memo.Content)

	// 3. 获取或创建同步状态
	syncState, err := s.stateTracker.GetSyncState(ctx, store.ContentTypeMemo, memoUID)
	if err != nil {
		return errors.Wrap(err, "获取同步状态失败")
	}

	// 4. 检查是否需要同步
	if syncState != nil && syncState.RAGFlowStatus == store.RAGFlowSyncStatusSynced {
		// 已同步，检查内容是否变更
		if syncState.ContentHash == contentHash {
			slog.Debug("Memo 内容未变更，跳过同步", slog.String("memoUID", memoUID))
			return nil
		}
		// 内容已变更，需要更新
		slog.Info("Memo 内容已变更，重新同步", slog.String("memoUID", memoUID))
	}

	// 5. 构建文档
	doc := s.buildMemoDocument(memo)

	// 6. 上传到 RAGFlow
	var docInfo *ragflow.DocumentInfo
	if syncState != nil && syncState.RAGFlowDocumentID != "" {
		// 已有文档，需要先删除再重新上传
		// RAGFlow 不支持直接更新文档内容，只能删除后重新上传
		if err := s.ragflowClient.DeleteDocuments(ctx, datasetID, []string{syncState.RAGFlowDocumentID}); err != nil {
			slog.Warn("删除旧文档失败，尝试创建新文档",
				slog.String("memoUID", memoUID),
				slog.Any("error", err))
		}
	}

	docInfo, err = s.ragflowClient.UploadDocument(ctx, datasetID, doc)
	if err != nil {
		return errors.Wrap(err, "上传文档到 RAGFlow 失败")
	}

	// 7. 触发解析
	if err := s.ragflowClient.ParseDocuments(ctx, datasetID, []string{docInfo.ID}); err != nil {
		slog.Warn("触发文档解析失败，文档已上传但未解析",
			slog.String("memoUID", memoUID),
			slog.Any("error", err))
		// 不返回错误，文档已上传成功，解析可以后续重试
	}

	// 8. 更新同步状态
	if syncState == nil {
		// 创建新状态
		_, err = s.stateTracker.CreatePendingState(ctx, store.ContentTypeMemo, memoUID, memo.CreatorID, contentHash)
		if err != nil {
			return errors.Wrap(err, "创建同步状态失败")
		}
		syncState, _ = s.stateTracker.GetSyncState(ctx, store.ContentTypeMemo, memoUID)
	}

	if err := s.stateTracker.MarkAsSynced(ctx, syncState.ID, datasetID, docInfo.ID); err != nil {
		return errors.Wrap(err, "更新同步状态失败")
	}

	// 更新内容哈希
	if err := s.updateContentHash(ctx, syncState.ID, contentHash); err != nil {
		slog.Warn("更新内容哈希失败", slog.Any("error", err))
	}

	slog.Info("Memo 同步成功",
		slog.String("memoUID", memoUID),
		slog.String("documentID", docInfo.ID))

	return nil
}

// DeleteMemoFromRAGFlow 从 RAGFlow 删除 Memo 文档
func (s *MemoSyncer) DeleteMemoFromRAGFlow(ctx context.Context, memoUID string) error {
	// 获取同步状态
	syncState, err := s.stateTracker.GetSyncState(ctx, store.ContentTypeMemo, memoUID)
	if err != nil {
		return errors.Wrap(err, "获取同步状态失败")
	}

	if syncState == nil {
		// 没有同步记录，无需删除
		return nil
	}

	if syncState.RAGFlowDocumentID != "" && syncState.RAGFlowDatasetID != "" {
		// 从 RAGFlow 删除文档
		if err := s.ragflowClient.DeleteDocuments(ctx, syncState.RAGFlowDatasetID, []string{syncState.RAGFlowDocumentID}); err != nil {
			slog.Warn("从 RAGFlow 删除文档失败",
				slog.String("memoUID", memoUID),
				slog.Any("error", err))
			// 继续删除本地同步状态
		}
	}

	// 删除同步状态
	if err := s.stateTracker.DeleteState(ctx, store.ContentTypeMemo, memoUID); err != nil {
		return errors.Wrap(err, "删除同步状态失败")
	}

	slog.Info("Memo 从 RAGFlow 删除成功", slog.String("memoUID", memoUID))
	return nil
}

// ==================== 批量同步方法 ====================

// SyncPendingMemos 同步所有待处理的 Memo
func (s *MemoSyncer) SyncPendingMemos(ctx context.Context, datasetIDGetter func(ownerID int32) (string, error), limit int) (int, int, error) {
	// 获取待同步的状态列表
	pendingStates, err := s.stateTracker.ListPendingStates(ctx, limit)
	if err != nil {
		return 0, 0, errors.Wrap(err, "获取待同步状态列表失败")
	}

	// 过滤出 Memo 类型
	memoStates := make([]*store.ContentSyncState, 0)
	for _, state := range pendingStates {
		if state.ContentType == store.ContentTypeMemo {
			memoStates = append(memoStates, state)
		}
	}

	if len(memoStates) == 0 {
		return 0, 0, nil
	}

	successCount := 0
	failCount := 0

	for _, state := range memoStates {
		// 获取用户的 Dataset ID
		datasetID, err := datasetIDGetter(state.OwnerID)
		if err != nil {
			slog.Error("获取用户 Dataset 失败",
				slog.Int("ownerID", int(state.OwnerID)),
				slog.Any("error", err))
			failCount++
			continue
		}

		// 同步 Memo
		if err := s.SyncMemo(ctx, state.ContentUID, datasetID); err != nil {
			slog.Error("同步 Memo 失败",
				slog.String("memoUID", state.ContentUID),
				slog.Any("error", err))
			// 标记为失败
			if markErr := s.stateTracker.MarkAsFailed(ctx, state.ID, state.RetryCount, err.Error()); markErr != nil {
				slog.Error("标记同步失败状态失败", slog.Any("error", markErr))
			}
			failCount++
			continue
		}

		successCount++
	}

	return successCount, failCount, nil
}

// ==================== 工具方法 ====================

// buildMemoDocument 构建 Memo 文档
func (s *MemoSyncer) buildMemoDocument(memo *store.Memo) *ragflow.Document {
	// 文档名格式: memo_{uid}.md
	docName := fmt.Sprintf("memo_%s.md", memo.UID)

	// 构建文档内容，包含元数据注释
	content := fmt.Sprintf(`---
memo_uid: %s
creator_id: %d
visibility: %s
created_ts: %d
---

%s`, memo.UID, memo.CreatorID, memo.Visibility, memo.CreatedTs, memo.Content)

	return &ragflow.Document{
		Name:     docName,
		Content:  []byte(content),
		MimeType: "text/markdown",
	}
}

// updateContentHash 更新内容哈希
func (s *MemoSyncer) updateContentHash(ctx context.Context, stateID int32, hash string) error {
	return s.store.UpdateContentSyncState(ctx, &store.UpdateContentSyncState{
		ID:          stateID,
		ContentHash: &hash,
	})
}

// CreateSyncStateForNewMemo 为新创建的 Memo 创建同步状态
func (s *MemoSyncer) CreateSyncStateForNewMemo(ctx context.Context, memo *store.Memo) error {
	contentHash := ComputeContentHash(memo.Content)
	_, err := s.stateTracker.CreatePendingState(ctx, store.ContentTypeMemo, memo.UID, memo.CreatorID, contentHash)
	return err
}

// MarkMemoForResync 标记 Memo 需要重新同步（内容更新时调用）
func (s *MemoSyncer) MarkMemoForResync(ctx context.Context, memoUID string, newContent string) error {
	state, err := s.stateTracker.GetSyncState(ctx, store.ContentTypeMemo, memoUID)
	if err != nil {
		return errors.Wrap(err, "获取同步状态失败")
	}

	newHash := ComputeContentHash(newContent)

	if state == nil {
		// 没有同步记录，创建新的
		memo, err := s.store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
		if err != nil {
			return errors.Wrap(err, "获取 Memo 失败")
		}
		if memo == nil {
			return errors.New("Memo 不存在")
		}
		_, err = s.stateTracker.CreatePendingState(ctx, store.ContentTypeMemo, memoUID, memo.CreatorID, newHash)
		return err
	}

	// 重置状态以便重新同步
	return s.stateTracker.ResetForResync(ctx, state.ID, newHash)
}
