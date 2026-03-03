package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateRAGFlowUserMapping(ctx context.Context, create *store.RAGFlowUserMapping) (*store.RAGFlowUserMapping, error) {
	fields := []string{"user_id", "dataset_id", "dataset_name", "assistant_id", "document_count", "ragflow_user_id", "ragflow_email", "ragflow_password", "api_key", "llm_configured", "preferred_llm_id", "dataset_ids", "quote_enabled", "reasoning_enabled"}
	placeholder := []string{"$1", "$2", "$3", "$4", "$5", "$6", "$7", "$8", "$9", "$10", "$11", "$12", "$13", "$14"}
	args := []any{create.UserID, create.DatasetID, create.DatasetName, create.AssistantID, create.DocumentCount, create.RAGFlowUserID, create.RAGFlowEmail, create.RAGFlowPassword, create.APIKey, create.LLMConfigured, create.PreferredLLMID, create.DatasetIDs, create.QuoteEnabled, create.ReasoningEnabled}

	if create.LastSyncTs != nil {
		fields = append(fields, "last_sync_ts")
		placeholder = append(placeholder, fmt.Sprintf("$%d", len(args)+1))
		args = append(args, *create.LastSyncTs)
	}

	stmt := "INSERT INTO ragflow_user_mapping (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING id, created_ts, updated_ts"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
		&create.UpdatedTs,
	); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListRAGFlowUserMappings(ctx context.Context, find *store.FindRAGFlowUserMapping) ([]*store.RAGFlowUserMapping, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, fmt.Sprintf("id = $%d", len(args)+1)), append(args, *find.ID)
	}
	if find.UserID != nil {
		where, args = append(where, fmt.Sprintf("user_id = $%d", len(args)+1)), append(args, *find.UserID)
	}
	if find.DatasetID != nil {
		where, args = append(where, fmt.Sprintf("dataset_id = $%d", len(args)+1)), append(args, *find.DatasetID)
	}

	query := "SELECT id, user_id, dataset_id, dataset_name, assistant_id, document_count, last_sync_ts, ragflow_user_id, ragflow_email, ragflow_password, api_key, llm_configured, preferred_llm_id, dataset_ids, quote_enabled, reasoning_enabled, created_ts, updated_ts FROM ragflow_user_mapping WHERE " + strings.Join(where, " AND ")

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
			&mapping.RAGFlowUserID,
			&mapping.RAGFlowEmail,
			&mapping.RAGFlowPassword,
			&mapping.APIKey,
			&mapping.LLMConfigured,
			&mapping.PreferredLLMID,
			&mapping.DatasetIDs,
			&mapping.QuoteEnabled,
			&mapping.ReasoningEnabled,
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
		set, args = append(set, fmt.Sprintf("dataset_id = $%d", len(args)+1)), append(args, *update.DatasetID)
	}
	if update.DatasetName != nil {
		set, args = append(set, fmt.Sprintf("dataset_name = $%d", len(args)+1)), append(args, *update.DatasetName)
	}
	if update.AssistantID != nil {
		set, args = append(set, fmt.Sprintf("assistant_id = $%d", len(args)+1)), append(args, *update.AssistantID)
	}
	if update.DocumentCount != nil {
		set, args = append(set, fmt.Sprintf("document_count = $%d", len(args)+1)), append(args, *update.DocumentCount)
	}
	if update.LastSyncTs != nil {
		set, args = append(set, fmt.Sprintf("last_sync_ts = $%d", len(args)+1)), append(args, *update.LastSyncTs)
	}
	if update.RAGFlowUserID != nil {
		set, args = append(set, fmt.Sprintf("ragflow_user_id = $%d", len(args)+1)), append(args, *update.RAGFlowUserID)
	}
	if update.RAGFlowEmail != nil {
		set, args = append(set, fmt.Sprintf("ragflow_email = $%d", len(args)+1)), append(args, *update.RAGFlowEmail)
	}
	if update.RAGFlowPassword != nil {
		set, args = append(set, fmt.Sprintf("ragflow_password = $%d", len(args)+1)), append(args, *update.RAGFlowPassword)
	}
	if update.APIKey != nil {
		set, args = append(set, fmt.Sprintf("api_key = $%d", len(args)+1)), append(args, *update.APIKey)
	}
	if update.LLMConfigured != nil {
		set, args = append(set, fmt.Sprintf("llm_configured = $%d", len(args)+1)), append(args, *update.LLMConfigured)
	}
	if update.PreferredLLMID != nil {
		set, args = append(set, fmt.Sprintf("preferred_llm_id = $%d", len(args)+1)), append(args, *update.PreferredLLMID)
	}
	if update.DatasetIDs != nil {
		set, args = append(set, fmt.Sprintf("dataset_ids = $%d", len(args)+1)), append(args, *update.DatasetIDs)
	}
	if update.QuoteEnabled != nil {
		set, args = append(set, fmt.Sprintf("quote_enabled = $%d", len(args)+1)), append(args, *update.QuoteEnabled)
	}
	if update.ReasoningEnabled != nil {
		set, args = append(set, fmt.Sprintf("reasoning_enabled = $%d", len(args)+1)), append(args, *update.ReasoningEnabled)
	}
	if update.UpdatedTs != nil {
		set, args = append(set, fmt.Sprintf("updated_ts = $%d", len(args)+1)), append(args, *update.UpdatedTs)
	}

	if len(set) == 0 {
		return nil
	}

	args = append(args, update.ID)
	stmt := fmt.Sprintf("UPDATE ragflow_user_mapping SET %s WHERE id = $%d", strings.Join(set, ", "), len(args))
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}

func (d *DB) DeleteRAGFlowUserMapping(ctx context.Context, delete *store.DeleteRAGFlowUserMapping) error {
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

	stmt := "DELETE FROM ragflow_user_mapping WHERE " + strings.Join(where, " AND ")
	_, err := d.db.ExecContext(ctx, stmt, args...)
	return err
}
