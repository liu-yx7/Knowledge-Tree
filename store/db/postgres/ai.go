package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateAIConversation(ctx context.Context, create *store.AIConversation) (*store.AIConversation, error) {
	fields := []string{"name", "creator_id", "llm_provider", "llm_model", "system_prompt"}
	placeholder := []string{"$1", "$2", "$3", "$4", "$5"}
	args := []any{create.Name, create.CreatorID, create.LLMProvider, create.LLMModel, create.SystemPrompt}

	stmt := "INSERT INTO ai_conversation (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING id, extract(epoch from created_ts)::bigint, extract(epoch from updated_ts)::bigint"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
		&create.UpdatedTs,
	); err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListAIConversations(ctx context.Context, find *store.FindAIConversation) ([]*store.AIConversation, error) {
	where, args := []string{"1 = 1"}, []any{}
	argIndex := 1

	if v := find.ID; v != nil {
		where = append(where, fmt.Sprintf("id = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := find.CreatorID; v != nil {
		where = append(where, fmt.Sprintf("creator_id = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}

	query := "SELECT id, name, creator_id, llm_provider, llm_model, system_prompt, extract(epoch from created_ts)::bigint, extract(epoch from updated_ts)::bigint " +
		"FROM ai_conversation " +
		"WHERE " + strings.Join(where, " AND ") + " " +
		"ORDER BY updated_ts DESC"

	if find.Limit != nil {
		query = fmt.Sprintf("%s LIMIT %d", query, *find.Limit)
		if find.Offset != nil {
			query = fmt.Sprintf("%s OFFSET %d", query, *find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*store.AIConversation, 0)
	for rows.Next() {
		var conversation store.AIConversation
		if err := rows.Scan(
			&conversation.ID,
			&conversation.Name,
			&conversation.CreatorID,
			&conversation.LLMProvider,
			&conversation.LLMModel,
			&conversation.SystemPrompt,
			&conversation.CreatedTs,
			&conversation.UpdatedTs,
		); err != nil {
			return nil, err
		}
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

	if v := update.Name; v != nil {
		set = append(set, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := update.SystemPrompt; v != nil {
		set = append(set, fmt.Sprintf("system_prompt = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := update.UpdatedTs; v != nil {
		set = append(set, fmt.Sprintf("updated_ts = to_timestamp($%d)", argIndex))
		args = append(args, *v)
		argIndex++
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := "UPDATE ai_conversation SET " + strings.Join(set, ", ") + fmt.Sprintf(" WHERE id = $%d", argIndex)
	if _, err := d.db.ExecContext(ctx, stmt, args...); err != nil {
		return err
	}

	return nil
}

func (d *DB) DeleteAIConversation(ctx context.Context, delete *store.DeleteAIConversation) error {
	stmt := "DELETE FROM ai_conversation WHERE id = $1"
	result, err := d.db.ExecContext(ctx, stmt, delete.ID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}

	return nil
}

func (d *DB) CreateAIMessage(ctx context.Context, create *store.AIMessage) (*store.AIMessage, error) {
	fields := []string{"conversation_id", "role", "content", "tokens"}
	placeholder := []string{"$1", "$2", "$3", "$4"}
	args := []any{create.ConversationID, create.Role, create.Content, create.Tokens}

	stmt := "INSERT INTO ai_message (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING id, extract(epoch from created_ts)::bigint"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
	); err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListAIMessages(ctx context.Context, find *store.FindAIMessage) ([]*store.AIMessage, error) {
	where, args := []string{"1 = 1"}, []any{}
	argIndex := 1

	if v := find.ID; v != nil {
		where = append(where, fmt.Sprintf("id = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := find.ConversationID; v != nil {
		where = append(where, fmt.Sprintf("conversation_id = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}

	query := "SELECT id, conversation_id, role, content, tokens, extract(epoch from created_ts)::bigint " +
		"FROM ai_message " +
		"WHERE " + strings.Join(where, " AND ") + " " +
		"ORDER BY created_ts ASC"

	if find.Limit != nil {
		query = fmt.Sprintf("%s LIMIT %d", query, *find.Limit)
		if find.Offset != nil {
			query = fmt.Sprintf("%s OFFSET %d", query, *find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*store.AIMessage, 0)
	for rows.Next() {
		var message store.AIMessage
		if err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.Role,
			&message.Content,
			&message.Tokens,
			&message.CreatedTs,
		); err != nil {
			return nil, err
		}
		list = append(list, &message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, nil
}

func (d *DB) DeleteAIMessage(ctx context.Context, delete *store.DeleteAIMessage) error {
	stmt := "DELETE FROM ai_message WHERE id = $1"
	result, err := d.db.ExecContext(ctx, stmt, delete.ID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}

	return nil
}

func (d *DB) CreateAIProviderConfig(ctx context.Context, create *store.AIProviderConfig) (*store.AIProviderConfig, error) {
	fields := []string{"name", "display_name", "api_key", "api_endpoint", "config", "enabled"}
	placeholder := []string{"$1", "$2", "$3", "$4", "$5", "$6"}
	args := []any{create.Name, create.DisplayName, create.APIKey, create.APIEndpoint, create.Config, create.Enabled}

	stmt := "INSERT INTO ai_provider_config (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING id, extract(epoch from created_ts)::bigint, extract(epoch from updated_ts)::bigint"
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
	argIndex := 1

	if v := find.ID; v != nil {
		where = append(where, fmt.Sprintf("id = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := find.Name; v != nil {
		where = append(where, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := find.Enabled; v != nil {
		where = append(where, fmt.Sprintf("enabled = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}

	query := "SELECT id, name, display_name, api_key, api_endpoint, config, enabled, extract(epoch from created_ts)::bigint, extract(epoch from updated_ts)::bigint " +
		"FROM ai_provider_config " +
		"WHERE " + strings.Join(where, " AND ") + " " +
		"ORDER BY name ASC"

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
	argIndex := 1

	if v := update.DisplayName; v != nil {
		set = append(set, fmt.Sprintf("display_name = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := update.APIKey; v != nil {
		set = append(set, fmt.Sprintf("api_key = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := update.APIEndpoint; v != nil {
		set = append(set, fmt.Sprintf("api_endpoint = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := update.Config; v != nil {
		set = append(set, fmt.Sprintf("config = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := update.Enabled; v != nil {
		set = append(set, fmt.Sprintf("enabled = $%d", argIndex))
		args = append(args, *v)
		argIndex++
	}
	if v := update.UpdatedTs; v != nil {
		set = append(set, fmt.Sprintf("updated_ts = to_timestamp($%d)", argIndex))
		args = append(args, *v)
		argIndex++
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := "UPDATE ai_provider_config SET " + strings.Join(set, ", ") + fmt.Sprintf(" WHERE id = $%d", argIndex)
	if _, err := d.db.ExecContext(ctx, stmt, args...); err != nil {
		return err
	}

	return nil
}

func (d *DB) DeleteAIProviderConfig(ctx context.Context, delete *store.DeleteAIProviderConfig) error {
	stmt := "DELETE FROM ai_provider_config WHERE id = $1"
	result, err := d.db.ExecContext(ctx, stmt, delete.ID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}

	return nil
}
