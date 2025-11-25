package sqlite

import (
	"context"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateAIProviderConfig(ctx context.Context, create *store.AIProviderConfig) (*store.AIProviderConfig, error) {
	fields := []string{"`name`", "`display_name`", "`api_key`", "`api_endpoint`", "`config`", "`enabled`"}
	placeholder := []string{"?", "?", "?", "?", "?", "?"}
	args := []any{create.Name, create.DisplayName, create.APIKey, create.APIEndpoint, create.Config, create.Enabled}

	stmt := "INSERT INTO `ai_provider_config` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING `id`, `created_ts`, `updated_ts`"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
		&create.UpdatedTs,
	); err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListAIProviderConfigs(ctx context.Context, find *store.FindAIProviderConfig) ([]*store.AIProviderConfig, error) {
	where, args := []string{"1 = 1"}, []any{}

	if v := find.ID; v != nil {
		where, args = append(where, "`id` = ?"), append(args, *v)
	}
	if v := find.Name; v != nil {
		where, args = append(where, "`name` = ?"), append(args, *v)
	}
	if v := find.Enabled; v != nil {
		where, args = append(where, "`enabled` = ?"), append(args, *v)
	}

	query := "SELECT `id`, `name`, `display_name`, `api_key`, `api_endpoint`, `config`, `enabled`, `created_ts`, `updated_ts` " +
		"FROM `ai_provider_config` " +
		"WHERE " + strings.Join(where, " AND ") + " " +
		"ORDER BY `name` ASC"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*store.AIProviderConfig, 0)
	for rows.Next() {
		var config store.AIProviderConfig
		if err := rows.Scan(
			&config.ID,
			&config.Name,
			&config.DisplayName,
			&config.APIKey,
			&config.APIEndpoint,
			&config.Config,
			&config.Enabled,
			&config.CreatedTs,
			&config.UpdatedTs,
		); err != nil {
			return nil, err
		}
		list = append(list, &config)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (d *DB) UpdateAIProviderConfig(ctx context.Context, update *store.UpdateAIProviderConfig) error {
	set, args := []string{}, []any{}

	if v := update.DisplayName; v != nil {
		set, args = append(set, "`display_name` = ?"), append(args, *v)
	}
	if v := update.APIKey; v != nil {
		set, args = append(set, "`api_key` = ?"), append(args, *v)
	}
	if v := update.APIEndpoint; v != nil {
		set, args = append(set, "`api_endpoint` = ?"), append(args, *v)
	}
	if v := update.Config; v != nil {
		set, args = append(set, "`config` = ?"), append(args, *v)
	}
	if v := update.Enabled; v != nil {
		set, args = append(set, "`enabled` = ?"), append(args, *v)
	}
	if v := update.UpdatedTs; v != nil {
		set, args = append(set, "`updated_ts` = ?"), append(args, *v)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := "UPDATE `ai_provider_config` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	if _, err := d.db.ExecContext(ctx, stmt, args...); err != nil {
		return err
	}

	return nil
}

func (d *DB) DeleteAIProviderConfig(ctx context.Context, delete *store.DeleteAIProviderConfig) error {
	stmt := "DELETE FROM `ai_provider_config` WHERE `id` = ?"
	result, err := d.db.ExecContext(ctx, stmt, delete.ID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}

	return nil
}
