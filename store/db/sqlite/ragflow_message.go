package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateRAGFlowMessage(ctx context.Context, create *store.RAGFlowMessage) (*store.RAGFlowMessage, error) {
	fields := []string{"`uid`", "`conversation_id`", "`role`", "`content`", "`references_json`"}
	placeholder := []string{"?", "?", "?", "?", "?"}
	args := []any{create.UID, create.ConversationID, create.Role, create.Content, create.ReferencesJSON}

	stmt := "INSERT INTO `ragflow_message` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING `id`, `created_ts`"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
	); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListRAGFlowMessages(ctx context.Context, find *store.FindRAGFlowMessage) ([]*store.RAGFlowMessage, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, "`uid` = ?"), append(args, *find.UID)
	}
	if find.ConversationID != nil {
		where, args = append(where, "`conversation_id` = ?"), append(args, *find.ConversationID)
	}

	orderBy := "DESC"
	if find.OrderByCreated != nil && *find.OrderByCreated == "ASC" {
		orderBy = "ASC"
	}

	query := "SELECT `id`, `uid`, `conversation_id`, `role`, `content`, `references_json`, `created_ts` FROM `ragflow_message` WHERE " + strings.Join(where, " AND ") + " ORDER BY `created_ts` " + orderBy

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

	list := []*store.RAGFlowMessage{}
	for rows.Next() {
		var message store.RAGFlowMessage
		var role string
		if err := rows.Scan(
			&message.ID,
			&message.UID,
			&message.ConversationID,
			&role,
			&message.Content,
			&message.ReferencesJSON,
			&message.CreatedTs,
		); err != nil {
			return nil, err
		}
		message.Role = store.RAGFlowMessageRole(role)
		list = append(list, &message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (d *DB) DeleteRAGFlowMessage(ctx context.Context, delete *store.DeleteRAGFlowMessage) error {
	where, args := []string{}, []any{}

	if delete.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *delete.ID)
	}
	if delete.ConversationID != nil {
		where, args = append(where, "`conversation_id` = ?"), append(args, *delete.ConversationID)
	}

	if len(where) == 0 {
		return fmt.Errorf("no filter specified for delete")
	}

	stmt := "DELETE FROM `ragflow_message` WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}
