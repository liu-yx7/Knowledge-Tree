package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateNotebook(ctx context.Context, create *store.Notebook) (*store.Notebook, error) {
	fields := []string{"uid", "creator_id", "title", "icon", "is_default", "dataset_id"}
	placeholder := []string{"$1", "$2", "$3", "$4", "$5", "$6"}
	args := []any{create.UID, create.CreatorID, create.Title, create.Icon, create.IsDefault, create.DatasetID}

	stmt := "INSERT INTO notebook (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING id, created_ts, updated_ts, row_status"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
		&create.UpdatedTs,
		&create.RowStatus,
	); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListNotebooks(ctx context.Context, find *store.FindNotebook) ([]*store.Notebook, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, fmt.Sprintf("uid = $%d", len(args)+1)), append(args, *find.UID)
	}
	if find.CreatorID != nil {
		where, args = append(where, fmt.Sprintf("creator_id = $%d", len(args)+1)), append(args, *find.CreatorID)
	}
	if find.IsDefault != nil {
		where, args = append(where, fmt.Sprintf("is_default = $%d", len(args)+1)), append(args, *find.IsDefault)
	}
	if find.RowStatus != nil {
		where, args = append(where, fmt.Sprintf("row_status = $%d", len(args)+1)), append(args, *find.RowStatus)
	}

	query := "SELECT id, uid, creator_id, title, icon, is_default, dataset_id, row_status, created_ts, updated_ts FROM notebook WHERE " + strings.Join(where, " AND ") + " ORDER BY is_default DESC, created_ts DESC"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*store.Notebook{}
	for rows.Next() {
		var notebook store.Notebook
		if err := rows.Scan(
			&notebook.ID,
			&notebook.UID,
			&notebook.CreatorID,
			&notebook.Title,
			&notebook.Icon,
			&notebook.IsDefault,
			&notebook.DatasetID,
			&notebook.RowStatus,
			&notebook.CreatedTs,
			&notebook.UpdatedTs,
		); err != nil {
			return nil, err
		}
		list = append(list, &notebook)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (d *DB) UpdateNotebook(ctx context.Context, update *store.UpdateNotebook) error {
	set, args := []string{}, []any{}

	if update.Title != nil {
		set, args = append(set, fmt.Sprintf("title = $%d", len(args)+1)), append(args, *update.Title)
	}
	if update.Icon != nil {
		set, args = append(set, fmt.Sprintf("icon = $%d", len(args)+1)), append(args, *update.Icon)
	}
	if update.DatasetID != nil {
		set, args = append(set, fmt.Sprintf("dataset_id = $%d", len(args)+1)), append(args, *update.DatasetID)
	}
	if update.RowStatus != nil {
		set, args = append(set, fmt.Sprintf("row_status = $%d", len(args)+1)), append(args, *update.RowStatus)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := fmt.Sprintf("UPDATE notebook SET %s WHERE id = $%d", strings.Join(set, ", "), len(args))
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteNotebook(ctx context.Context, delete *store.DeleteNotebook) error {
	where, args := []string{fmt.Sprintf("id = $%d", 1)}, []any{delete.ID}
	stmt := "DELETE FROM notebook WHERE " + strings.Join(where, " AND ")
	result, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}
	return nil
}
