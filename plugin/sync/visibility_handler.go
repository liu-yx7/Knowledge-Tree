package sync

import (
	"context"
	"log/slog"

	"github.com/pkg/errors"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// ==================== Visibility Handler 定义 ====================

// VisibilityHandler 处理 Memo visibility 变更
// 职责：当 Memo 的 visibility 变更时，更新 RAGFlow Document 的元数据
type VisibilityHandler struct {
	store         *store.Store
	ragflowClient *ragflow.Client
	stateTracker  *StateTracker
}

// NewVisibilityHandler 创建 visibility 处理器
func NewVisibilityHandler(s *store.Store, client *ragflow.Client, tracker *StateTracker) *VisibilityHandler {
	return &VisibilityHandler{
		store:         s,
		ragflowClient: client,
		stateTracker:  tracker,
	}
}

// SetClient 替换当前使用的 RAGFlow 客户端（用于注入 per-user Client）
func (h *VisibilityHandler) SetClient(client *ragflow.Client) {
	h.ragflowClient = client
}

// ==================== 核心方法 ====================

// HandleVisibilityChange 处理 visibility 变更
// 当 Memo 从 PUBLIC ↔ PRIVATE 切换时，更新 RAGFlow Document 元数据
func (h *VisibilityHandler) HandleVisibilityChange(ctx context.Context, memoUID string, newVisibility store.Visibility) error {
	// 1. 获取同步状态
	syncState, err := h.stateTracker.GetSyncState(ctx, store.ContentTypeMemo, memoUID)
	if err != nil {
		return errors.Wrap(err, "获取同步状态失败")
	}

	// 2. 检查是否已同步
	if syncState == nil {
		// 没有同步记录，下次同步时会使用新的 visibility
		slog.Debug("Memo 尚未同步到 RAGFlow，跳过 visibility 更新",
			slog.String("memoUID", memoUID))
		return nil
	}

	if syncState.RAGFlowStatus != store.RAGFlowSyncStatusSynced {
		// 不是已同步状态，跳过
		slog.Debug("Memo 不在已同步状态，跳过 visibility 更新",
			slog.String("memoUID", memoUID),
			slog.String("status", string(syncState.RAGFlowStatus)))
		return nil
	}

	if syncState.RAGFlowDocumentID == "" || syncState.RAGFlowDatasetID == "" {
		// 没有 RAGFlow 文档信息
		slog.Warn("Memo 缺少 RAGFlow 文档信息，无法更新 visibility",
			slog.String("memoUID", memoUID))
		return nil
	}

	// 3. 更新 RAGFlow Document 元数据
	metadata := map[string]any{
		"visibility": string(newVisibility),
	}

	_, err = h.ragflowClient.UpdateDocument(ctx, syncState.RAGFlowDatasetID, syncState.RAGFlowDocumentID, &ragflow.UpdateDocumentRequest{
		Metadata: metadata,
	})
	if err != nil {
		return errors.Wrap(err, "更新 RAGFlow Document 元数据失败")
	}

	slog.Info("Memo visibility 更新成功",
		slog.String("memoUID", memoUID),
		slog.String("newVisibility", string(newVisibility)))

	return nil
}

// ==================== 批量更新方法 ====================

// BatchUpdateVisibility 批量更新多个 Memo 的 visibility
func (h *VisibilityHandler) BatchUpdateVisibility(ctx context.Context, memoUIDs []string, newVisibility store.Visibility) (int, int, error) {
	successCount := 0
	failCount := 0

	for _, memoUID := range memoUIDs {
		if err := h.HandleVisibilityChange(ctx, memoUID, newVisibility); err != nil {
			slog.Error("更新 Memo visibility 失败",
				slog.String("memoUID", memoUID),
				slog.Any("error", err))
			failCount++
			continue
		}
		successCount++
	}

	return successCount, failCount, nil
}

// ==================== 检索过滤辅助 ====================

// FilterByVisibility 根据 visibility 过滤检索结果
// 请求者是内容所有者时，可见所有内容
// 请求者是其他用户时，只能看到 PUBLIC 内容
func (h *VisibilityHandler) FilterByVisibility(ctx context.Context, chunks []ragflow.Chunk, requesterID int32) ([]ragflow.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	filtered := make([]ragflow.Chunk, 0, len(chunks))

	for _, chunk := range chunks {
		// 从文档名解析 memo UID
		memoUID := extractMemoUIDFromDocumentName(chunk.DocumentName)
		if memoUID == "" {
			// 无法解析，跳过
			continue
		}

		// 获取 Memo
		memo, err := h.store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
		if err != nil || memo == nil {
			// 获取失败或不存在，跳过
			continue
		}

		// 检查可见性
		if memo.CreatorID == requesterID {
			// 所有者可以看到所有内容
			filtered = append(filtered, chunk)
		} else if memo.Visibility == store.Public {
			// 非所有者只能看到 PUBLIC 内容
			filtered = append(filtered, chunk)
		}
		// PRIVATE 和 PROTECTED 内容对非所有者不可见
	}

	return filtered, nil
}

// extractMemoUIDFromDocumentName 从文档名提取 Memo UID
// 文档名格式: memo_{uid}.md
func extractMemoUIDFromDocumentName(docName string) string {
	// 检查是否以 "memo_" 开头
	if len(docName) < 6 || docName[:5] != "memo_" {
		return ""
	}

	// 去掉 "memo_" 前缀和 ".md" 后缀
	name := docName[5:]
	if len(name) > 3 && name[len(name)-3:] == ".md" {
		name = name[:len(name)-3]
	}

	return name
}

// ==================== 附件 visibility 处理 ====================

// HandleAttachmentVisibilityChange 处理附件关联的 Memo visibility 变更
// 附件的可见性跟随其关联的 Memo
func (h *VisibilityHandler) HandleAttachmentVisibilityChange(ctx context.Context, memoUID string, newVisibility store.Visibility) error {
	// 获取 Memo 关联的所有附件
	memo, err := h.store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return errors.Wrap(err, "获取 Memo 失败")
	}
	if memo == nil {
		return errors.New("Memo 不存在")
	}

	attachments, err := h.store.ListAttachments(ctx, &store.FindAttachment{MemoID: &memo.ID})
	if err != nil {
		return errors.Wrap(err, "获取附件列表失败")
	}

	// 更新每个附件的 visibility
	for _, attachment := range attachments {
		syncState, err := h.stateTracker.GetSyncState(ctx, store.ContentTypeAttachment, attachment.UID)
		if err != nil {
			slog.Warn("获取附件同步状态失败",
				slog.String("attachmentUID", attachment.UID),
				slog.Any("error", err))
			continue
		}

		if syncState == nil || syncState.RAGFlowStatus != store.RAGFlowSyncStatusSynced {
			continue
		}

		if syncState.RAGFlowDocumentID == "" || syncState.RAGFlowDatasetID == "" {
			continue
		}

		metadata := map[string]any{
			"visibility": string(newVisibility),
		}

		_, err = h.ragflowClient.UpdateDocument(ctx, syncState.RAGFlowDatasetID, syncState.RAGFlowDocumentID, &ragflow.UpdateDocumentRequest{
			Metadata: metadata,
		})
		if err != nil {
			slog.Warn("更新附件 visibility 失败",
				slog.String("attachmentUID", attachment.UID),
				slog.Any("error", err))
			continue
		}

		slog.Debug("附件 visibility 更新成功",
			slog.String("attachmentUID", attachment.UID),
			slog.String("newVisibility", string(newVisibility)))
	}

	return nil
}
