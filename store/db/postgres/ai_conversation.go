package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateAIConversation(ctx context.Context, create *store.AIConversation) (*store.AIConversation, error) {
	fields := []string{"uid", "user_id", "title"}
	args := []any{create.UID, create.UserID, create.Title}

	stmt := "INSERT INTO ai_conversation (" + strings.Join(fields, ", ") + ") VALUES ($1, $2, $3) RETURNING id, created_ts, updated_ts, row_status"
	var rowStatus string
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
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

func (d *DB) ListAIConversations(ctx context.Context, find *store.FindAIConversation) ([]*store.AIConversation, error) {
	where, args := []string{"1 = 1"}, []any{}
	argIndex := 1

	if find.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", argIndex)), append(args, *find.ID)
		argIndex++
	}
	if find.UID != nil {
		where, args = append(where, fmt.Sprintf("uid = $%d", argIndex)), append(args, *find.UID)
		argIndex++
	}
	if find.UserID != nil {
		where, args = append(where, fmt.Sprintf("user_id = $%d", argIndex)), append(args, *find.UserID)
		argIndex++
	}
	if find.RowStatus != nil {
		where, args = append(where, fmt.Sprintf("row_status = $%d", argIndex)), append(args, *find.RowStatus)
		argIndex++
	}

	query := "SELECT id, uid, user_id, title, created_ts, updated_ts, row_status FROM ai_conversation WHERE " + strings.Join(where, " AND ") + " ORDER BY updated_ts DESC"

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

	list := []*store.AIConversation{}
	for rows.Next() {
		var conversation store.AIConversation
		var rowStatus string
		if err := rows.Scan(
			&conversation.ID,
			&conversation.UID,
			&conversation.UserID,
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

func (d *DB) UpdateAIConversation(ctx context.Context, update *store.UpdateAIConversation) error {
	set, args := []string{}, []any{}
	argIndex := 1

	if update.Title != nil {
		set, args = append(set, fmt.Sprintf("title = $%d", argIndex)), append(args, *update.Title)
		argIndex++
	}
	if update.RowStatus != nil {
		set, args = append(set, fmt.Sprintf("row_status = $%d", argIndex)), append(args, *update.RowStatus)
		argIndex++
	}
	if update.UpdatedTs != nil {
		set, args = append(set, fmt.Sprintf("updated_ts = $%d", argIndex)), append(args, *update.UpdatedTs)
		argIndex++
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := fmt.Sprintf("UPDATE ai_conversation SET %s WHERE id = $%d", strings.Join(set, ", "), argIndex)
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteAIConversation(ctx context.Context, delete *store.DeleteAIConversation) error {
	where, args := []string{"id = $1"}, []any{delete.ID}
	argIndex := 2

	if delete.UserID != nil {
		where, args = append(where, fmt.Sprintf("user_id = $%d", argIndex)), append(args, *delete.UserID)
	}

	stmt := "DELETE FROM ai_conversation WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}
