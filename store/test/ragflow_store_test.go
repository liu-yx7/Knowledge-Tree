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

func TestRAGFlowConversationStore(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// 创建测试用户
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	// ==================== 测试 Create ====================
	conversation := &store.RAGFlowConversation{
		UID:              "conv_uid_123",
		UserID:           user.ID,
		RAGFlowSessionID: "session_456",
		Title:            "Test Conversation",
	}
	createdConv, err := ts.CreateRAGFlowConversation(ctx, conversation)
	require.NoError(t, err)
	assert.NotZero(t, createdConv.ID)
	assert.Equal(t, "conv_uid_123", createdConv.UID)
	assert.Equal(t, store.Normal, createdConv.RowStatus)

	// ==================== 测试 Get ====================
	foundConv, err := ts.GetRAGFlowConversation(ctx, &store.FindRAGFlowConversation{
		UID: &conversation.UID,
	})
	require.NoError(t, err)
	require.NotNil(t, foundConv)
	assert.Equal(t, createdConv.ID, foundConv.ID)

	// ==================== 测试 List ====================
	conversations, err := ts.ListRAGFlowConversations(ctx, &store.FindRAGFlowConversation{
		UserID: &user.ID,
	})
	require.NoError(t, err)
	assert.Len(t, conversations, 1)

	// ==================== 测试 Update ====================
	newTitle := "Updated Title"
	err = ts.UpdateRAGFlowConversation(ctx, &store.UpdateRAGFlowConversation{
		ID:    createdConv.ID,
		Title: &newTitle,
	})
	require.NoError(t, err)

	updatedConv, err := ts.GetRAGFlowConversation(ctx, &store.FindRAGFlowConversation{
		ID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updatedConv.Title)

	// ==================== 测试 Delete ====================
	err = ts.DeleteRAGFlowConversation(ctx, &store.DeleteRAGFlowConversation{
		ID: &createdConv.ID,
	})
	require.NoError(t, err)

	deletedConv, err := ts.GetRAGFlowConversation(ctx, &store.FindRAGFlowConversation{
		ID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Nil(t, deletedConv)
}

func TestRAGFlowMessageStore(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	// 创建测试用户和对话
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	conversation := &store.RAGFlowConversation{
		UID:              "conv_msg_test",
		UserID:           user.ID,
		RAGFlowSessionID: "session_msg_test",
		Title:            "Message Test Conversation",
	}
	createdConv, err := ts.CreateRAGFlowConversation(ctx, conversation)
	require.NoError(t, err)

	// ==================== 测试 Create ====================
	message := &store.RAGFlowMessage{
		UID:            "msg_uid_123",
		ConversationID: createdConv.ID,
		Role:           store.RAGFlowMessageRoleUser,
		Content:        "Hello, this is a test message",
		ReferencesJSON: "[]",
	}
	createdMsg, err := ts.CreateRAGFlowMessage(ctx, message)
	require.NoError(t, err)
	assert.NotZero(t, createdMsg.ID)
	assert.Equal(t, store.RAGFlowMessageRoleUser, createdMsg.Role)

	// 创建助手回复
	assistantMsg := &store.RAGFlowMessage{
		UID:            "msg_uid_456",
		ConversationID: createdConv.ID,
		Role:           store.RAGFlowMessageRoleAssistant,
		Content:        "Hello! How can I help you?",
		ReferencesJSON: `[{"memo_uid":"memo_1","content_snippet":"test snippet","similarity_score":0.95}]`,
	}
	createdAssistantMsg, err := ts.CreateRAGFlowMessage(ctx, assistantMsg)
	require.NoError(t, err)
	assert.Equal(t, store.RAGFlowMessageRoleAssistant, createdAssistantMsg.Role)

	// ==================== 测试 Get ====================
	foundMsg, err := ts.GetRAGFlowMessage(ctx, &store.FindRAGFlowMessage{
		UID: &message.UID,
	})
	require.NoError(t, err)
	require.NotNil(t, foundMsg)
	assert.Equal(t, createdMsg.ID, foundMsg.ID)

	// ==================== 测试 List ====================
	messages, err := ts.ListRAGFlowMessages(ctx, &store.FindRAGFlowMessage{
		ConversationID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Len(t, messages, 2)

	// 测试排序
	orderASC := "ASC"
	messagesASC, err := ts.ListRAGFlowMessages(ctx, &store.FindRAGFlowMessage{
		ConversationID: &createdConv.ID,
		OrderByCreated: &orderASC,
	})
	require.NoError(t, err)
	assert.Equal(t, store.RAGFlowMessageRoleUser, messagesASC[0].Role) // 用户消息在前

	// ==================== 测试 Delete by ConversationID ====================
	err = ts.DeleteRAGFlowMessage(ctx, &store.DeleteRAGFlowMessage{
		ConversationID: &createdConv.ID,
	})
	require.NoError(t, err)

	remainingMsgs, err := ts.ListRAGFlowMessages(ctx, &store.FindRAGFlowMessage{
		ConversationID: &createdConv.ID,
	})
	require.NoError(t, err)
	assert.Len(t, remainingMsgs, 0)
}
