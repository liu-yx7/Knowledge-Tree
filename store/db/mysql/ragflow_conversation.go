package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateRAGFlowConversation(ctx context.Context, create *store.RAGFlowConversation) (*store.RAGFlowConversation, error) {
	fields := []string{"`uid`", "`user_id`", "`ragflow_session_id`", "`title`"}
	placeholder := []string{"?", "?", "?", "?"}
	args := []any{create.UID, create.UserID, create.RAGFlowSessionID, create.Title}

	stmt := "INSERT INTO `ragflow_conversation` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ")"
	result, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	create.ID = int32(id)

	var rowStatus string
	if err := d.db.QueryRowContext(ctx, "SELECT `created_ts`, `updated_ts`, `row_status` FROM `ragflow_conversation` WHERE `id` = ?", create.ID).Scan(&create.CreatedTs, &create.UpdatedTs, &rowStatus); err != nil {
		return nil, err
	}
	create.RowStatus = store.RowStatus(rowStatus)

	return create, nil
}

func (d *DB) ListRAGFlowConversations(ctx context.Context, find *store.FindRAGFlowConversation) ([]*store.RAGFlowConversation, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, "`uid` = ?"), append(args, *find.UID)
	}
	if find.UserID != nil {
		where, args = append(where, "`user_id` = ?"), append(args, *find.UserID)
	}
	if find.RAGFlowSessionID != nil {
		where, args = append(where, "`ragflow_session_id` = ?"), append(args, *find.RAGFlowSessionID)
	}
	if find.RowStatus != nil {
		where, args = append(where, "`row_status` = ?"), append(args, *find.RowStatus)
	}

	query := "SELECT `id`, `uid`, `user_id`, `ragflow_session_id`, `title`, `created_ts`, `updated_ts`, `row_status` FROM `ragflow_conversation` WHERE " + strings.Join(where, " AND ") + " ORDER BY `updated_ts` DESC"

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

	list := []*store.RAGFlowConversation{}
	for rows.Next() {
		var conversation store.RAGFlowConversation
		var rowStatus string
		if err := rows.Scan(
			&conversation.ID,
			&conversation.UID,
			&conversation.UserID,
			&conversation.RAGFlowSessionID,
			&conversation.Title,
			&conversation.CreatedTs,
			&conversation.UpdatedTs,
			&rowStatus,
		); err != nil {
			return nil, err
		}
		conversation.RowStatus = store.RowStatus(rowStatus)
		list = append(list, &conversation)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (d *DB) UpdateRAGFlowConversation(ctx context.Context, update *store.UpdateRAGFlowConversation) error {
	set, args := []string{}, []any{}

	if update.Title != nil {
		set, args = append(set, "`title` = ?"), append(args, *update.Title)
	}
	if update.RowStatus != nil {
		set, args = append(set, "`row_status` = ?"), append(args, *update.RowStatus)
	}
	if update.UpdatedTs != nil {
		set, args = append(set, "`updated_ts` = ?"), append(args, *update.UpdatedTs)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := "UPDATE `ragflow_conversation` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteRAGFlowConversation(ctx context.Context, delete *store.DeleteRAGFlowConversation) error {
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

	stmt := "DELETE FROM `ragflow_conversation` WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}
