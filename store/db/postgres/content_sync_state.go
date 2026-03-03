package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateContentSyncState(ctx context.Context, create *store.ContentSyncState) (*store.ContentSyncState, error) {
	fields := []string{"content_type", "content_uid", "owner_id", "ragflow_status", "ragflow_dataset_id", "ragflow_document_id", "ragflow_error", "content_hash", "retry_count"}
	placeholder := []string{"$1", "$2", "$3", "$4", "$5", "$6", "$7", "$8", "$9"}
	args := []any{create.ContentType, create.ContentUID, create.OwnerID, create.RAGFlowStatus, create.RAGFlowDatasetID, create.RAGFlowDocumentID, create.RAGFlowError, create.ContentHash, create.RetryCount}

	if create.RAGFlowSyncedTs != nil {
		fields = append(fields, "ragflow_synced_ts")
		placeholder = append(placeholder, fmt.Sprintf("$%d", len(args)+1))
		args = append(args, *create.RAGFlowSyncedTs)
	}
	if create.NextRetryTs != nil {
		fields = append(fields, "next_retry_ts")
		placeholder = append(placeholder, fmt.Sprintf("$%d", len(args)+1))
		args = append(args, *create.NextRetryTs)
	}

	stmt := "INSERT INTO content_sync_state (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING id, created_ts, updated_ts"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
		&create.UpdatedTs,
	); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListContentSyncStates(ctx context.Context, find *store.FindContentSyncState) ([]*store.ContentSyncState, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, *find.ID)
	}
	if find.ContentType != nil {
		where, args = append(where, fmt.Sprintf("content_type = $%d", len(args)+1)), append(args, *find.ContentType)
	}
	if find.ContentUID != nil {
		where, args = append(where, fmt.Sprintf("content_uid = $%d", len(args)+1)), append(args, *find.ContentUID)
	}
	if find.OwnerID != nil {
		where, args = append(where, fmt.Sprintf("owner_id = $%d", len(args)+1)), append(args, *find.OwnerID)
	}
	if find.RAGFlowStatus != nil {
		where, args = append(where, fmt.Sprintf("ragflow_status = $%d", len(args)+1)), append(args, *find.RAGFlowStatus)
	}

	query := "SELECT id, content_type, content_uid, owner_id, ragflow_status, ragflow_dataset_id, ragflow_document_id, ragflow_synced_ts, ragflow_error, content_hash, retry_count, next_retry_ts, created_ts, updated_ts FROM content_sync_state WHERE " + strings.Join(where, " AND ") + " ORDER BY created_ts DESC"

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
		set, args = append(set, fmt.Sprintf("ragflow_status = $%d", len(args)+1)), append(args, *update.RAGFlowStatus)
	}
	if update.RAGFlowDatasetID != nil {
		set, args = append(set, fmt.Sprintf("ragflow_dataset_id = $%d", len(args)+1)), append(args, *update.RAGFlowDatasetID)
	}
	if update.RAGFlowDocumentID != nil {
		set, args = append(set, fmt.Sprintf("ragflow_document_id = $%d", len(args)+1)), append(args, *update.RAGFlowDocumentID)
	}
	if update.RAGFlowSyncedTs != nil {
		set, args = append(set, fmt.Sprintf("ragflow_synced_ts = $%d", len(args)+1)), append(args, *update.RAGFlowSyncedTs)
	}
	if update.RAGFlowError != nil {
		set, args = append(set, fmt.Sprintf("ragflow_error = $%d", len(args)+1)), append(args, *update.RAGFlowError)
	}
	if update.ContentHash != nil {
		set, args = append(set, fmt.Sprintf("content_hash = $%d", len(args)+1)), append(args, *update.ContentHash)
	}
	if update.RetryCount != nil {
		set, args = append(set, fmt.Sprintf("retry_count = $%d", len(args)+1)), append(args, *update.RetryCount)
	}
	if update.NextRetryTs != nil {
		set, args = append(set, fmt.Sprintf("next_retry_ts = $%d", len(args)+1)), append(args, *update.NextRetryTs)
	}
	if update.UpdatedTs != nil {
		set, args = append(set, fmt.Sprintf("updated_ts = $%d", len(args)+1)), append(args, *update.UpdatedTs)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := fmt.Sprintf("UPDATE content_sync_state SET %s WHERE id = $%d", strings.Join(set, ", "), len(args))
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteContentSyncState(ctx context.Context, delete *store.DeleteContentSyncState) error {
	where, args := []string{}, []any{}

	if delete.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, *delete.ID)
	}
	if delete.ContentType != nil {
		where, args = append(where, fmt.Sprintf("content_type = $%d", len(args)+1)), append(args, *delete.ContentType)
	}
	if delete.ContentUID != nil {
		where, args = append(where, fmt.Sprintf("content_uid = $%d", len(args)+1)), append(args, *delete.ContentUID)
	}
	if delete.OwnerID != nil {
		where, args = append(where, fmt.Sprintf("owner_id = $%d", len(args)+1)), append(args, *delete.OwnerID)
	}

	if len(where) == 0 {
		return fmt.Errorf("no filter specified for delete")
	}

	stmt := "DELETE FROM content_sync_state WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) UpsertContentSyncState(ctx context.Context, create *store.ContentSyncState) (*store.ContentSyncState, error) {
	stmt := `
		INSERT INTO content_sync_state (content_type, content_uid, owner_id, ragflow_status, ragflow_dataset_id, ragflow_document_id, ragflow_synced_ts, ragflow_error, content_hash, retry_count, next_retry_ts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT(content_type, content_uid) DO UPDATE SET
			ragflow_status = EXCLUDED.ragflow_status,
			ragflow_dataset_id = EXCLUDED.ragflow_dataset_id,
			ragflow_document_id = EXCLUDED.ragflow_document_id,
			ragflow_synced_ts = EXCLUDED.ragflow_synced_ts,
			ragflow_error = EXCLUDED.ragflow_error,
			content_hash = EXCLUDED.content_hash,
			retry_count = EXCLUDED.retry_count,
			next_retry_ts = EXCLUDED.next_retry_ts,
			updated_ts = EXTRACT(EPOCH FROM NOW())
		RETURNING id, created_ts, updated_ts
	`
	if err := d.db.QueryRowContext(ctx, stmt,
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
	).Scan(&create.ID, &create.CreatedTs, &create.UpdatedTs); err != nil {
		return nil, err
	}
	return create, nil
}
