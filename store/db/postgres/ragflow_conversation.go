package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateRAGFlowConversation(ctx context.Context, create *store.RAGFlowConversation) (*store.RAGFlowConversation, error) {
	stmt := "INSERT INTO ragflow_conversation (uid, user_id, ragflow_session_id, title) VALUES ($1, $2, $3, $4) RETURNING id, created_ts, updated_ts, row_status"
	var rowStatus string
	if err := d.db.QueryRowContext(ctx, stmt, create.UID, create.UserID, create.RAGFlowSessionID, create.Title).Scan(
		&create.ID,
		&create.CreatedTs,
		&create.UpdatedTs,
		&rowStatus,
	); err != nil {
		return nil, err
	}
	create.RowStatus = store.RowStatus(rowStatus)
	return create, nil
}

func (d *DB) ListRAGFlowConversations(ctx context.Context, find *store.FindRAGFlowConversation) ([]*store.RAGFlowConversation, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, fmt.Sprintf("uid = $%d", len(args)+1)), append(args, *find.UID)
	}
	if find.UserID != nil {
		where, args = append(where, fmt.Sprintf("user_id = $%d", len(args)+1)), append(args, *find.UserID)
	}
	if find.RAGFlowSessionID != nil {
		where, args = append(where, fmt.Sprintf("ragflow_session_id = $%d", len(args)+1)), append(args, *find.RAGFlowSessionID)
	}
	if find.RowStatus != nil {
		where, args = append(where, fmt.Sprintf("row_status = $%d", len(args)+1)), append(args, *find.RowStatus)
	}

	query := "SELECT id, uid, user_id, ragflow_session_id, title, created_ts, updated_ts, row_status FROM ragflow_conversation WHERE " + strings.Join(where, " AND ") + " ORDER BY updated_ts DESC"

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
		set, args = append(set, fmt.Sprintf("title = $%d", len(args)+1)), append(args, *update.Title)
	}
	if update.RowStatus != nil {
		set, args = append(set, fmt.Sprintf("row_status = $%d", len(args)+1)), append(args, *update.RowStatus)
	}
	if update.UpdatedTs != nil {
		set, args = append(set, fmt.Sprintf("updated_ts = $%d", len(args)+1)), append(args, *update.UpdatedTs)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := fmt.Sprintf("UPDATE ragflow_conversation SET %s WHERE id = $%d", strings.Join(set, ", "), len(args))
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteRAGFlowConversation(ctx context.Context, delete *store.DeleteRAGFlowConversation) error {
	where, args := []string{}, []any{}

	if delete.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, *delete.ID)
	}
	if delete.UserID != nil {
		where, args = append(where, fmt.Sprintf("user_id = $%d", len(args)+1)), append(args, *delete.UserID)
	}

	if len(where) == 0 {
		return fmt.Errorf("no filter specified for delete")
	}

	stmt := "DELETE FROM ragflow_conversation WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}
