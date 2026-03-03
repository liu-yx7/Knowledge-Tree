package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateContentSyncState(ctx context.Context, create *store.ContentSyncState) (*store.ContentSyncState, error) {
	fields := []string{"`content_type`", "`content_uid`", "`owner_id`", "`ragflow_status`", "`ragflow_dataset_id`", "`ragflow_document_id`", "`ragflow_error`", "`content_hash`", "`retry_count`"}
	placeholder := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?"}
	args := []any{create.ContentType, create.ContentUID, create.OwnerID, create.RAGFlowStatus, create.RAGFlowDatasetID, create.RAGFlowDocumentID, create.RAGFlowError, create.ContentHash, create.RetryCount}

	if create.RAGFlowSyncedTs != nil {
		fields = append(fields, "`ragflow_synced_ts`")
		placeholder = append(placeholder, "?")
		args = append(args, *create.RAGFlowSyncedTs)
	}
	if create.NextRetryTs != nil {
		fields = append(fields, "`next_retry_ts`")
		placeholder = append(placeholder, "?")
		args = append(args, *create.NextRetryTs)
	}

	stmt := "INSERT INTO `content_sync_state` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ")"
	result, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	create.ID = int32(id)

	if err := d.db.QueryRowContext(ctx, "SELECT `created_ts`, `updated_ts` FROM `content_sync_state` WHERE `id` = ?", create.ID).Scan(&create.CreatedTs, &create.UpdatedTs); err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListContentSyncStates(ctx context.Context, find *store.FindContentSyncState) ([]*store.ContentSyncState, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *find.ID)
	}
	if find.ContentType != nil {
		where, args = append(where, "`content_type` = ?"), append(args, *find.ContentType)
	}
	if find.ContentUID != nil {
		where, args = append(where, "`content_uid` = ?"), append(args, *find.ContentUID)
	}
	if find.OwnerID != nil {
		where, args = append(where, "`owner_id` = ?"), append(args, *find.OwnerID)
	}
	if find.RAGFlowStatus != nil {
		where, args = append(where, "`ragflow_status` = ?"), append(args, *find.RAGFlowStatus)
	}

	query := "SELECT `id`, `content_type`, `content_uid`, `owner_id`, `ragflow_status`, `ragflow_dataset_id`, `ragflow_document_id`, `ragflow_synced_ts`, `ragflow_error`, `content_hash`, `retry_count`, `next_retry_ts`, `created_ts`, `updated_ts` FROM `content_sync_state` WHERE " + strings.Join(where, " AND ") + " ORDER BY `created_ts` DESC"

	if find.Limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *find.Limit)
		if find.Offset != nil {
			query += fmt.Sprintf(" OFFSET %d", *find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*store.ContentSyncState{}
	for rows.Next() {
		var state store.ContentSyncState
		var contentType, ragflowStatus string
		if err := rows.Scan(
			&state.ID,
			&contentType,
			&state.ContentUID,
			&state.OwnerID,
			&ragflowStatus,
			&state.RAGFlowDatasetID,
			&state.RAGFlowDocumentID,
			&state.RAGFlowSyncedTs,
			&state.RAGFlowError,
			&state.ContentHash,
			&state.RetryCount,
			&state.NextRetryTs,
			&state.CreatedTs,
			&state.UpdatedTs,
		); err != nil {
			return nil, err
		}
		state.ContentType = store.ContentType(contentType)
		state.RAGFlowStatus = store.RAGFlowSyncStatus(ragflowStatus)
		list = append(list, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (d *DB) UpdateContentSyncState(ctx context.Context, update *store.UpdateContentSyncState) error {
	set, args := []string{}, []any{}

	if update.RAGFlowStatus != nil {
		set, args = append(set, "`ragflow_status` = ?"), append(args, *update.RAGFlowStatus)
	}
	if update.RAGFlowDatasetID != nil {
		set, args = append(set, "`ragflow_dataset_id` = ?"), append(args, *update.RAGFlowDatasetID)
	}
	if update.RAGFlowDocumentID != nil {
		set, args = append(set, "`ragflow_document_id` = ?"), append(args, *update.RAGFlowDocumentID)
	}
	if update.RAGFlowSyncedTs != nil {
		set, args = append(set, "`ragflow_synced_ts` = ?"), append(args, *update.RAGFlowSyncedTs)
	}
	if update.RAGFlowError != nil {
		set, args = append(set, "`ragflow_error` = ?"), append(args, *update.RAGFlowError)
	}
	if update.ContentHash != nil {
		set, args = append(set, "`content_hash` = ?"), append(args, *update.ContentHash)
	}
	if update.RetryCount != nil {
		set, args = append(set, "`retry_count` = ?"), append(args, *update.RetryCount)
	}
	if update.NextRetryTs != nil {
		set, args = append(set, "`next_retry_ts` = ?"), append(args, *update.NextRetryTs)
	}
	if update.UpdatedTs != nil {
		set, args = append(set, "`updated_ts` = ?"), append(args, *update.UpdatedTs)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := "UPDATE `content_sync_state` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteContentSyncState(ctx context.Context, delete *store.DeleteContentSyncState) error {
	where, args := []string{}, []any{}

	if delete.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *delete.ID)
	}
	if delete.ContentType != nil {
		where, args = append(where, "`content_type` = ?"), append(args, *delete.ContentType)
	}
	if delete.ContentUID != nil {
		where, args = append(where, "`content_uid` = ?"), append(args, *delete.ContentUID)
	}
	if delete.OwnerID != nil {
		where, args = append(where, "`owner_id` = ?"), append(args, *delete.OwnerID)
	}

	if len(where) == 0 {
		return fmt.Errorf("no filter specified for delete")
	}

	stmt := "DELETE FROM `content_sync_state` WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) UpsertContentSyncState(ctx context.Context, create *store.ContentSyncState) (*store.ContentSyncState, error) {
	stmt := `
		INSERT INTO content_sync_state (content_type, content_uid, owner_id, ragflow_status, ragflow_dataset_id, ragflow_document_id, ragflow_synced_ts, ragflow_error, content_hash, retry_count, next_retry_ts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			ragflow_status = VALUES(ragflow_status),
			ragflow_dataset_id = VALUES(ragflow_dataset_id),
			ragflow_document_id = VALUES(ragflow_document_id),
			ragflow_synced_ts = VALUES(ragflow_synced_ts),
			ragflow_error = VALUES(ragflow_error),
			content_hash = VALUES(content_hash),
			retry_count = VALUES(retry_count),
			next_retry_ts = VALUES(next_retry_ts),
			updated_ts = UNIX_TIMESTAMP()
	`
	result, err := d.db.ExecContext(ctx, stmt,
		create.ContentType,
		create.ContentUID,
		create.OwnerID,
		create.RAGFlowStatus,
		create.RAGFlowDatasetID,
		create.RAGFlowDocumentID,
		create.RAGFlowSyncedTs,
		create.RAGFlowError,
		create.ContentHash,
		create.RetryCount,
		create.NextRetryTs,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	if id != 0 {
		create.ID = int32(id)
	}

	// 查询获取完整记录
	if err := d.db.QueryRowContext(ctx, "SELECT `id`, `created_ts`, `updated_ts` FROM `content_sync_state` WHERE `content_type` = ? AND `content_uid` = ?", create.ContentType, create.ContentUID).Scan(&create.ID, &create.CreatedTs, &create.UpdatedTs); err != nil {
		return nil, err
	}

	return create, nil
}
