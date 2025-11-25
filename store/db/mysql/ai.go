package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateAIConversation(ctx context.Context, create *store.AIConversation) (*store.AIConversation, error) {
	fields := []string{"`name`", "`creator_id`", "`llm_provider`", "`llm_model`", "`system_prompt`"}
	placeholder := []string{"?", "?", "?", "?", "?"}
	args := []any{create.Name, create.CreatorID, create.LLMProvider, create.LLMModel, create.SystemPrompt}

	stmt := "INSERT INTO `ai_conversation` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ")"
	result, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	rawID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	id := int32(rawID)
	
	// Get the created conversation
	conversation, err := d.GetAIConversation(ctx, &store.FindAIConversation{ID: &id})
	if err != nil {
		return nil, err
	}
	return conversation, nil
}

func (d *DB) ListAIConversations(ctx context.Context, find *store.FindAIConversation) ([]*store.AIConversation, error) {
	where, args := []string{"1 = 1"}, []any{}

	if v := find.ID; v != nil {
		where, args = append(where, "`id` = ?"), append(args, *v)
	}
	if v := find.CreatorID; v != nil {
		where, args = append(where, "`creator_id` = ?"), append(args, *v)
	}

	query := "SELECT `id`, `name`, `creator_id`, `llm_provider`, `llm_model`, `system_prompt`, UNIX_TIMESTAMP(`created_ts`) AS `created_ts`, UNIX_TIMESTAMP(`updated_ts`) AS `updated_ts` " +
		"FROM `ai_conversation` " +
		"WHERE " + strings.Join(where, " AND ") + " " +
		"ORDER BY `updated_ts` DESC"

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

func (d *DB) GetAIConversation(ctx context.Context, find *store.FindAIConversation) (*store.AIConversation, error) {
	list, err := d.ListAIConversations(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (d *DB) UpdateAIConversation(ctx context.Context, update *store.UpdateAIConversation) error {
	set, args := []string{}, []any{}

	if v := update.Name; v != nil {
		set, args = append(set, "`name` = ?"), append(args, *v)
	}
	if v := update.SystemPrompt; v != nil {
		set, args = append(set, "`system_prompt` = ?"), append(args, *v)
	}
	if v := update.UpdatedTs; v != nil {
		set, args = append(set, "`updated_ts` = FROM_UNIXTIME(?)"), append(args, *v)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := "UPDATE `ai_conversation` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	if _, err := d.db.ExecContext(ctx, stmt, args...); err != nil {
		return err
	}

	return nil
}

func (d *DB) DeleteAIConversation(ctx context.Context, delete *store.DeleteAIConversation) error {
	stmt := "DELETE FROM `ai_conversation` WHERE `id` = ?"
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
	fields := []string{"`conversation_id`", "`role`", "`content`", "`tokens`"}
	placeholder := []string{"?", "?", "?", "?"}
	args := []any{create.ConversationID, create.Role, create.Content, create.Tokens}

	stmt := "INSERT INTO `ai_message` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ")"
	result, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	rawID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	id := int32(rawID)
	
	message, err := d.GetAIMessage(ctx, &store.FindAIMessage{ID: &id})
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (d *DB) ListAIMessages(ctx context.Context, find *store.FindAIMessage) ([]*store.AIMessage, error) {
	where, args := []string{"1 = 1"}, []any{}

	if v := find.ID; v != nil {
		where, args = append(where, "`id` = ?"), append(args, *v)
	}
	if v := find.ConversationID; v != nil {
		where, args = append(where, "`conversation_id` = ?"), append(args, *v)
	}

	query := "SELECT `id`, `conversation_id`, `role`, `content`, `tokens`, UNIX_TIMESTAMP(`created_ts`) AS `created_ts` " +
		"FROM `ai_message` " +
		"WHERE " + strings.Join(where, " AND ") + " " +
		"ORDER BY `created_ts` ASC"

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

func (d *DB) GetAIMessage(ctx context.Context, find *store.FindAIMessage) (*store.AIMessage, error) {
	list, err := d.ListAIMessages(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (d *DB) DeleteAIMessage(ctx context.Context, delete *store.DeleteAIMessage) error {
	stmt := "DELETE FROM `ai_message` WHERE `id` = ?"
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
	fields := []string{"`name`", "`display_name`", "`api_key`", "`api_endpoint`", "`config`", "`enabled`"}
	placeholder := []string{"?", "?", "?", "?", "?", "?"}
	args := []any{create.Name, create.DisplayName, create.APIKey, create.APIEndpoint, create.Config, create.Enabled}

	stmt := "INSERT INTO `ai_provider_config` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ")"
	result, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	rawID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	id := int32(rawID)
	
	config, err := d.GetAIProviderConfig(ctx, &store.FindAIProviderConfig{ID: &id})
	if err != nil {
		return nil, err
	}
	return config, nil
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

	query := "SELECT `id`, `name`, `display_name`, `api_key`, `api_endpoint`, `config`, `enabled`, UNIX_TIMESTAMP(`created_ts`) AS `created_ts`, UNIX_TIMESTAMP(`updated_ts`) AS `updated_ts` " +
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

func (d *DB) GetAIProviderConfig(ctx context.Context, find *store.FindAIProviderConfig) (*store.AIProviderConfig, error) {
	list, err := d.ListAIProviderConfigs(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
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
		set, args = append(set, "`updated_ts` = FROM_UNIXTIME(?)"), append(args, *v)
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
