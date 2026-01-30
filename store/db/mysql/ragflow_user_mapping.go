package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateRAGFlowUserMapping(ctx context.Context, create *store.RAGFlowUserMapping) (*store.RAGFlowUserMapping, error) {
	fields := []string{"`user_id`", "`dataset_id`", "`dataset_name`", "`assistant_id`", "`document_count`"}
	placeholder := []string{"?", "?", "?", "?", "?"}
	args := []any{create.UserID, create.DatasetID, create.DatasetName, create.AssistantID, create.DocumentCount}

	if create.LastSyncTs != nil {
		fields = append(fields, "`last_sync_ts`")
		placeholder = append(placeholder, "?")
		args = append(args, *create.LastSyncTs)
	}

	stmt := "INSERT INTO `ragflow_user_mapping` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ")"
	result, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	create.ID = int32(id)

	// 获取时间戳
	if err := d.db.QueryRowContext(ctx, "SELECT `created_ts`, `updated_ts` FROM `ragflow_user_mapping` WHERE `id` = ?", create.ID).Scan(&create.CreatedTs, &create.UpdatedTs); err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListRAGFlowUserMappings(ctx context.Context, find *store.FindRAGFlowUserMapping) ([]*store.RAGFlowUserMapping, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *find.ID)
	}
	if find.UserID != nil {
		where, args = append(where, "`user_id` = ?"), append(args, *find.UserID)
	}
	if find.DatasetID != nil {
		where, args = append(where, "`dataset_id` = ?"), append(args, *find.DatasetID)
	}

	query := "SELECT `id`, `user_id`, `dataset_id`, `dataset_name`, `assistant_id`, `document_count`, `last_sync_ts`, `created_ts`, `updated_ts` FROM `ragflow_user_mapping` WHERE " + strings.Join(where, " AND ")

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*store.RAGFlowUserMapping{}
	for rows.Next() {
		var mapping store.RAGFlowUserMapping
		if err := rows.Scan(
			&mapping.ID,
			&mapping.UserID,
			&mapping.DatasetID,
			&mapping.DatasetName,
			&mapping.AssistantID,
			&mapping.DocumentCount,
			&mapping.LastSyncTs,
			&mapping.CreatedTs,
			&mapping.UpdatedTs,
		); err != nil {
			return nil, err
		}
		list = append(list, &mapping)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (d *DB) UpdateRAGFlowUserMapping(ctx context.Context, update *store.UpdateRAGFlowUserMapping) error {
	set, args := []string{}, []any{}

	if update.DatasetID != nil {
		set, args = append(set, "`dataset_id` = ?"), append(args, *update.DatasetID)
	}
	if update.DatasetName != nil {
		set, args = append(set, "`dataset_name` = ?"), append(args, *update.DatasetName)
	}
	if update.AssistantID != nil {
		set, args = append(set, "`assistant_id` = ?"), append(args, *update.AssistantID)
	}
	if update.DocumentCount != nil {
		set, args = append(set, "`document_count` = ?"), append(args, *update.DocumentCount)
	}
	if update.LastSyncTs != nil {
		set, args = append(set, "`last_sync_ts` = ?"), append(args, *update.LastSyncTs)
	}
	if update.UpdatedTs != nil {
		set, args = append(set, "`updated_ts` = ?"), append(args, *update.UpdatedTs)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := "UPDATE `ragflow_user_mapping` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteRAGFlowUserMapping(ctx context.Context, delete *store.DeleteRAGFlowUserMapping) error {
	where, args := []string{}, []any{}

	if delete.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *delete.ID)
	}
	if delete.UserID != nil {
		where, args = append(where, "`user_id` = ?"), append(args, *delete.UserID)
	}

	if len(where) == 0 {
		return fmt.Errorf("no filter specified for delete")
	}

	stmt := "DELETE FROM `ragflow_user_mapping` WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}
