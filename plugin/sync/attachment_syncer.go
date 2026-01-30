package sync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pkg/errors"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// ==================== Attachment Syncer 定义 ====================

// AttachmentSyncer 附件同步器
// 职责：将可解析的附件同步到 RAGFlow Dataset
type AttachmentSyncer struct {
	store         *store.Store
	ragflowClient *ragflow.Client
	stateTracker  *StateTracker
}

// NewAttachmentSyncer 创建附件同步器
func NewAttachmentSyncer(s *store.Store, client *ragflow.Client, tracker *StateTracker) *AttachmentSyncer {
	return &AttachmentSyncer{
		store:         s,
		ragflowClient: client,
		stateTracker:  tracker,
	}
}

// ==================== 可解析附件类型判断 ====================

// parseableMimeTypes 可被 RAGFlow 解析的 MIME 类型
var parseableMimeTypes = map[string]bool{
	// 文档类型
	"application/pdf":    true,
	"text/plain":         true,
	"text/markdown":      true,
	"text/html":          true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	// 图片类型（支持 OCR）
	"image/png":  true,
	"image/jpeg": true,
	"image/jpg":  true,
	"image/tiff": true,
	"image/bmp":  true,
	"image/webp": true,
}

// IsParseableAttachment 判断附件是否可被 RAGFlow 解析
func IsParseableAttachment(mimeType string) bool {
	return parseableMimeTypes[mimeType]
}

// ==================== 同步方法 ====================

// SyncAttachment 同步单个附件到 RAGFlow
// 流程：
// 1. 获取附件信息
// 2. 检查是否为可解析类型
// 3. 获取附件内容
// 4. 上传到 RAGFlow
// 5. 触发解析
// 6. 更新同步状态
func (s *AttachmentSyncer) SyncAttachment(ctx context.Context, attachmentUID string, datasetID string) error {
	// 1. 获取附件信息
	attachment, err := s.store.GetAttachment(ctx, &store.FindAttachment{UID: &attachmentUID})
	if err != nil {
		return errors.Wrap(err, "获取附件失败")
	}
	if attachment == nil {
		return errors.New("附件不存在")
	}

	// 2. 检查是否为可解析类型
	if !IsParseableAttachment(attachment.Type) {
		// 标记为跳过
		state, _ := s.stateTracker.GetSyncState(ctx, store.ContentTypeAttachment, attachmentUID)
		if state != nil {
			if err := s.stateTracker.MarkAsSkipped(ctx, state.ID, "不支持的附件类型: "+attachment.Type); err != nil {
				slog.Warn("标记附件为跳过状态失败", slog.Any("error", err))
			}
		}
		slog.Debug("附件类型不支持解析，跳过同步",
			slog.String("attachmentUID", attachmentUID),
			slog.String("type", attachment.Type))
		return nil
	}

	// 3. 获取附件内容
	attachmentWithBlob, err := s.store.GetAttachment(ctx, &store.FindAttachment{
		UID:     &attachmentUID,
		GetBlob: true,
	})
	if err != nil {
		return errors.Wrap(err, "获取附件内容失败")
	}
	if attachmentWithBlob == nil || len(attachmentWithBlob.Blob) == 0 {
		return errors.New("附件内容为空")
	}

	// 4. 获取或创建同步状态
	syncState, err := s.stateTracker.GetSyncState(ctx, store.ContentTypeAttachment, attachmentUID)
	if err != nil {
		return errors.Wrap(err, "获取同步状态失败")
	}

	// 计算内容哈希
	contentHash := ComputeContentHash(string(attachmentWithBlob.Blob))

	// 检查是否需要同步
	if syncState != nil && syncState.RAGFlowStatus == store.RAGFlowSyncStatusSynced {
		if syncState.ContentHash == contentHash {
			slog.Debug("附件内容未变更，跳过同步", slog.String("attachmentUID", attachmentUID))
			return nil
		}
		slog.Info("附件内容已变更，重新同步", slog.String("attachmentUID", attachmentUID))
	}

	// 5. 构建文档
	doc := s.buildAttachmentDocument(attachmentWithBlob)

	// 6. 上传到 RAGFlow
	var docInfo *ragflow.DocumentInfo
	if syncState != nil && syncState.RAGFlowDocumentID != "" {
		// 已有文档，需要先删除再重新上传
		if err := s.ragflowClient.DeleteDocuments(ctx, datasetID, []string{syncState.RAGFlowDocumentID}); err != nil {
			slog.Warn("删除旧附件文档失败，尝试创建新文档",
				slog.String("attachmentUID", attachmentUID),
				slog.Any("error", err))
		}
	}

	docInfo, err = s.ragflowClient.UploadDocument(ctx, datasetID, doc)
	if err != nil {
		return errors.Wrap(err, "上传附件到 RAGFlow 失败")
	}

	// 7. 触发解析
	if err := s.ragflowClient.ParseDocuments(ctx, datasetID, []string{docInfo.ID}); err != nil {
		slog.Warn("触发附件解析失败，文档已上传但未解析",
			slog.String("attachmentUID", attachmentUID),
			slog.Any("error", err))
	}

	// 8. 更新同步状态
	if syncState == nil {
		_, err = s.stateTracker.CreatePendingState(ctx, store.ContentTypeAttachment, attachmentUID, attachment.CreatorID, contentHash)
		if err != nil {
			return errors.Wrap(err, "创建同步状态失败")
		}
		syncState, _ = s.stateTracker.GetSyncState(ctx, store.ContentTypeAttachment, attachmentUID)
	}

	if err := s.stateTracker.MarkAsSynced(ctx, syncState.ID, datasetID, docInfo.ID); err != nil {
		return errors.Wrap(err, "更新同步状态失败")
	}

	// 更新内容哈希
	if err := s.updateContentHash(ctx, syncState.ID, contentHash); err != nil {
		slog.Warn("更新内容哈希失败", slog.Any("error", err))
	}

	slog.Info("附件同步成功",
		slog.String("attachmentUID", attachmentUID),
		slog.String("documentID", docInfo.ID))

	return nil
}

// DeleteAttachmentFromRAGFlow 从 RAGFlow 删除附件文档
func (s *AttachmentSyncer) DeleteAttachmentFromRAGFlow(ctx context.Context, attachmentUID string) error {
	syncState, err := s.stateTracker.GetSyncState(ctx, store.ContentTypeAttachment, attachmentUID)
	if err != nil {
		return errors.Wrap(err, "获取同步状态失败")
	}

	if syncState == nil {
		return nil
	}

	if syncState.RAGFlowDocumentID != "" && syncState.RAGFlowDatasetID != "" {
		if err := s.ragflowClient.DeleteDocuments(ctx, syncState.RAGFlowDatasetID, []string{syncState.RAGFlowDocumentID}); err != nil {
			slog.Warn("从 RAGFlow 删除附件文档失败",
				slog.String("attachmentUID", attachmentUID),
				slog.Any("error", err))
		}
	}

	if err := s.stateTracker.DeleteState(ctx, store.ContentTypeAttachment, attachmentUID); err != nil {
		return errors.Wrap(err, "删除同步状态失败")
	}

	slog.Info("附件从 RAGFlow 删除成功", slog.String("attachmentUID", attachmentUID))
	return nil
}

// ==================== 批量同步方法 ====================

// SyncPendingAttachments 同步所有待处理的附件
func (s *AttachmentSyncer) SyncPendingAttachments(ctx context.Context, datasetIDGetter func(ownerID int32) (string, error), limit int) (int, int, error) {
	pendingStates, err := s.stateTracker.ListPendingStates(ctx, limit)
	if err != nil {
		return 0, 0, errors.Wrap(err, "获取待同步状态列表失败")
	}

	// 过滤出附件类型
	attachmentStates := make([]*store.ContentSyncState, 0)
	for _, state := range pendingStates {
		if state.ContentType == store.ContentTypeAttachment {
			attachmentStates = append(attachmentStates, state)
		}
	}

	if len(attachmentStates) == 0 {
		return 0, 0, nil
	}

	successCount := 0
	failCount := 0

	for _, state := range attachmentStates {
		datasetID, err := datasetIDGetter(state.OwnerID)
		if err != nil {
			slog.Error("获取用户 Dataset 失败",
				slog.Int("ownerID", int(state.OwnerID)),
				slog.Any("error", err))
			failCount++
			continue
		}

		if err := s.SyncAttachment(ctx, state.ContentUID, datasetID); err != nil {
			slog.Error("同步附件失败",
				slog.String("attachmentUID", state.ContentUID),
				slog.Any("error", err))
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

// buildAttachmentDocument 构建附件文档
func (s *AttachmentSyncer) buildAttachmentDocument(attachment *store.Attachment) *ragflow.Document {
	// 文档名格式: attachment_{uid}_{filename}
	docName := fmt.Sprintf("attachment_%s_%s", attachment.UID, attachment.Filename)

	return &ragflow.Document{
		Name:     docName,
		Content:  attachment.Blob,
		MimeType: attachment.Type,
	}
}

// updateContentHash 更新内容哈希
func (s *AttachmentSyncer) updateContentHash(ctx context.Context, stateID int32, hash string) error {
	return s.store.UpdateContentSyncState(ctx, &store.UpdateContentSyncState{
		ID:          stateID,
		ContentHash: &hash,
	})
}

// CreateSyncStateForNewAttachment 为新创建的附件创建同步状态
func (s *AttachmentSyncer) CreateSyncStateForNewAttachment(ctx context.Context, attachment *store.Attachment) error {
	// 检查是否为可解析类型
	if !IsParseableAttachment(attachment.Type) {
		// 创建跳过状态
		state, err := s.stateTracker.CreatePendingState(ctx, store.ContentTypeAttachment, attachment.UID, attachment.CreatorID, "")
		if err != nil {
			return err
		}
		return s.stateTracker.MarkAsSkipped(ctx, state.ID, "不支持的附件类型: "+attachment.Type)
	}

	// 创建待同步状态（不计算哈希，等到实际同步时再计算）
	_, err := s.stateTracker.CreatePendingState(ctx, store.ContentTypeAttachment, attachment.UID, attachment.CreatorID, "")
	return err
}
