package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestRAGFlowUserMappingStore(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// 创建测试用户
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	// ==================== 测试 Create ====================
	mapping := &store.RAGFlowUserMapping{
		UserID:        user.ID,
		DatasetID:     "dataset_123",
		DatasetName:   "Test Dataset",
		AssistantID:   "assistant_456",
		DocumentCount: 10,
	}
	createdMapping, err := ts.CreateRAGFlowUserMapping(ctx, mapping)
	require.NoError(t, err)
	assert.NotZero(t, createdMapping.ID)
	assert.Equal(t, user.ID, createdMapping.UserID)
	assert.Equal(t, "dataset_123", createdMapping.DatasetID)
	assert.Equal(t, "Test Dataset", createdMapping.DatasetName)
	assert.NotZero(t, createdMapping.CreatedTs)

	// ==================== 测试 Get ====================
	foundMapping, err := ts.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		UserID: &user.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, foundMapping)
	assert.Equal(t, createdMapping.ID, foundMapping.ID)
	assert.Equal(t, "dataset_123", foundMapping.DatasetID)

	// ==================== 测试 Update ====================
	newDatasetName := "Updated Dataset"
	newDocCount := int32(20)
	err = ts.UpdateRAGFlowUserMapping(ctx, &store.UpdateRAGFlowUserMapping{
		ID:            createdMapping.ID,
		DatasetName:   &newDatasetName,
		DocumentCount: &newDocCount,
	})
	require.NoError(t, err)

	updatedMapping, err := ts.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		ID: &createdMapping.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Dataset", updatedMapping.DatasetName)
	assert.Equal(t, int32(20), updatedMapping.DocumentCount)

	// ==================== 测试 Delete ====================
	err = ts.DeleteRAGFlowUserMapping(ctx, &store.DeleteRAGFlowUserMapping{
		ID: &createdMapping.ID,
	})
	require.NoError(t, err)

	deletedMapping, err := ts.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{
		ID: &createdMapping.ID,
	})
	require.NoError(t, err)
	assert.Nil(t, deletedMapping)
}

func TestContentSyncStateStore(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// 创建测试用户
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	// ==================== 测试 Create ====================
	syncState := &store.ContentSyncState{
		ContentType:       store.ContentTypeMemo,
		ContentUID:        "memo_uid_123",
		OwnerID:           user.ID,
		RAGFlowStatus:     store.RAGFlowSyncStatusPending,
		RAGFlowDatasetID:  "dataset_123",
		RAGFlowDocumentID: "",
		RAGFlowError:      "",
		ContentHash:       "abc123hash",
		RetryCount:        0,
	}
	createdState, err := ts.CreateContentSyncState(ctx, syncState)
	require.NoError(t, err)
	assert.NotZero(t, createdState.ID)
	assert.Equal(t, store.ContentTypeMemo, createdState.ContentType)
	assert.Equal(t, store.RAGFlowSyncStatusPending, createdState.RAGFlowStatus)

	// ==================== 测试 Get ====================
	contentType := store.ContentTypeMemo
	foundState, err := ts.GetContentSyncState(ctx, &store.FindContentSyncState{
		ContentType: &contentType,
		ContentUID:  &syncState.ContentUID,
	})
	require.NoError(t, err)
	require.NotNil(t, foundState)
	assert.Equal(t, createdState.ID, foundState.ID)

	// ==================== 测试 Update ====================
	newStatus := store.RAGFlowSyncStatusSynced
	newDocID := "doc_789"
	err = ts.UpdateContentSyncState(ctx, &store.UpdateContentSyncState{
		ID:                createdState.ID,
		RAGFlowStatus:     &newStatus,
		RAGFlowDocumentID: &newDocID,
	})
	require.NoError(t, err)

	updatedState, err := ts.GetContentSyncState(ctx, &store.FindContentSyncState{
		ID: &createdState.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, store.RAGFlowSyncStatusSynced, updatedState.RAGFlowStatus)
	assert.Equal(t, "doc_789", updatedState.RAGFlowDocumentID)

	// ==================== 测试 Upsert ====================
	upsertState := &store.ContentSyncState{
		ContentType:       store.ContentTypeMemo,
		ContentUID:        "memo_uid_123", // 同一个 UID
		OwnerID:           user.ID,
		RAGFlowStatus:     store.RAGFlowSyncStatusFailed,
		RAGFlowDatasetID:  "dataset_123",
		RAGFlowDocumentID: "doc_789",
		RAGFlowError:      "test error",
		ContentHash:       "new_hash",
		RetryCount:        1,
	}
	upsertedState, err := ts.UpsertContentSyncState(ctx, upsertState)
	require.NoError(t, err)
	assert.Equal(t, createdState.ID, upsertedState.ID) // 应该是同一条记录
	assert.Equal(t, store.RAGFlowSyncStatusFailed, upsertedState.RAGFlowStatus)

	// ==================== 测试 Delete ====================
	err = ts.DeleteContentSyncState(ctx, &store.DeleteContentSyncState{
		ID: &createdState.ID,
	})
	require.NoError(t, err)

	deletedState, err := ts.GetContentSyncState(ctx, &store.FindContentSyncState{
		ID: &createdState.ID,
	})
	require.NoError(t, err)
	assert.Nil(t, deletedState)
}

func TestAIConversationStore(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// 创建测试用户
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	// ==================== 测试 Create ====================
	conversation := &store.AIConversation{
		UID:    "conv_uid_123",
		UserID: user.ID,
		Title:  "Test Conversation",
	}
	createdConv, err := ts.CreateAIConversation(ctx, conversation)
	require.NoError(t, err)
	assert.NotZero(t, createdConv.ID)
	assert.Equal(t, "conv_uid_123", createdConv.UID)
	assert.Equal(t, store.Normal, createdConv.RowStatus)

	// ==================== 测试 Get ====================
	foundConv, err := ts.GetAIConversation(ctx, &store.FindAIConversation{
		UID: &conversation.UID,
	})
	require.NoError(t, err)
	require.NotNil(t, foundConv)
	assert.Equal(t, createdConv.ID, foundConv.ID)

	// ==================== 测试 List ====================
	conversations, err := ts.ListAIConversations(ctx, &store.FindAIConversation{
		UserID: &user.ID,
	})
	require.NoError(t, err)
	assert.Len(t, conversations, 1)

	// ==================== 测试 Update ====================
	newTitle := "Updated Title"
	err = ts.UpdateAIConversation(ctx, &store.UpdateAIConversation{
		ID:    createdConv.ID,
		Title: &newTitle,
	})
	require.NoError(t, err)

	updatedConv, err := ts.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updatedConv.Title)

	// ==================== 测试 Delete ====================
	err = ts.DeleteAIConversation(ctx, &store.DeleteAIConversation{
		ID: createdConv.ID,
	})
	require.NoError(t, err)

	deletedConv, err := ts.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Nil(t, deletedConv)
}

func TestAIMessageStore(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// 创建测试用户和对话
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	conversation := &store.AIConversation{
		UID:    "conv_msg_test",
		UserID: user.ID,
		Title:  "Message Test Conversation",
	}
	createdConv, err := ts.CreateAIConversation(ctx, conversation)
	require.NoError(t, err)

	// ==================== 测试 Create ====================
	message := &store.AIMessage{
		UID:            "msg_uid_123",
		ConversationID: createdConv.ID,
		Role:           store.AIMessageRoleUser,
		Content:        "Hello, this is a test message",
	}
	createdMsg, err := ts.CreateAIMessage(ctx, message)
	require.NoError(t, err)
	assert.NotZero(t, createdMsg.ID)
	assert.Equal(t, store.AIMessageRoleUser, createdMsg.Role)

	// 创建助手回复（含引用和思考链）
	assistantMsg := &store.AIMessage{
		UID:              "msg_uid_456",
		ConversationID:   createdConv.ID,
		Role:             store.AIMessageRoleAssistant,
		Content:          "Hello! How can I help you?",
		ReasoningContent: "The user is greeting me, I should respond politely.",
		ReferencesJSON:   `[{"memo_uid":"memo_1","type":"memo","content_snippet":"test snippet","similarity":0.95}]`,
		TokenUsageJSON:   `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}`,
	}
	createdAssistantMsg, err := ts.CreateAIMessage(ctx, assistantMsg)
	require.NoError(t, err)
	assert.Equal(t, store.AIMessageRoleAssistant, createdAssistantMsg.Role)

	// ==================== 测试 List ====================
	messages, err := ts.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Len(t, messages, 2)

	// 测试排序
	orderASC := "ASC"
	messagesASC, err := ts.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &createdConv.ID,
		OrderByCreated: &orderASC,
	})
	require.NoError(t, err)
	assert.Equal(t, store.AIMessageRoleUser, messagesASC[0].Role) // 用户消息在前

	// 验证引用和思考链字段
	assert.NotEmpty(t, messagesASC[1].ReferencesJSON)
	assert.NotEmpty(t, messagesASC[1].ReasoningContent)
	assert.NotEmpty(t, messagesASC[1].TokenUsageJSON)

	// ==================== 测试 Delete by ConversationID ====================
	err = ts.DeleteAIMessage(ctx, &store.DeleteAIMessage{
		ConversationID: &createdConv.ID,
	})
	require.NoError(t, err)

	remainingMsgs, err := ts.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Len(t, remainingMsgs, 0)
}
