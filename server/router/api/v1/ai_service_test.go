package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// ==================== buildOpenAIMessages 测试 ====================

func TestBuildOpenAIMessages_EmptyHistory(t *testing.T) {
	messages := buildOpenAIMessages(nil, "hello")
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "hello", messages[0].Content)
}

func TestBuildOpenAIMessages_WithHistory(t *testing.T) {
	history := []*store.AIMessage{
		{Role: store.AIMessageRoleUser, Content: "question 1"},
		{Role: store.AIMessageRoleAssistant, Content: "answer 1"},
		{Role: store.AIMessageRoleUser, Content: "question 2"},
		{Role: store.AIMessageRoleAssistant, Content: "answer 2"},
	}

	messages := buildOpenAIMessages(history, "question 3")
	require.Len(t, messages, 5)

	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "question 1", messages[0].Content)
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "answer 1", messages[1].Content)
	assert.Equal(t, "user", messages[2].Role)
	assert.Equal(t, "question 2", messages[2].Content)
	assert.Equal(t, "assistant", messages[3].Role)
	assert.Equal(t, "answer 2", messages[3].Content)
	assert.Equal(t, "user", messages[4].Role)
	assert.Equal(t, "question 3", messages[4].Content)
}

func TestBuildOpenAIMessages_TruncatesLongHistory(t *testing.T) {
	// 创建超过 maxHistoryMessages 的历史
	history := make([]*store.AIMessage, 0, 30)
	for i := 0; i < 30; i++ {
		role := store.AIMessageRoleUser
		if i%2 == 1 {
			role = store.AIMessageRoleAssistant
		}
		history = append(history, &store.AIMessage{
			Role:    role,
			Content: "msg",
		})
	}

	messages := buildOpenAIMessages(history, "new question")

	// 应该只有 maxHistoryMessages + 1（新消息）条
	assert.Equal(t, maxHistoryMessages+1, len(messages))

	// 最后一条应该是新消息
	last := messages[len(messages)-1]
	assert.Equal(t, "user", last.Role)
	assert.Equal(t, "new question", last.Content)
}

func TestBuildOpenAIMessages_ExactlyMaxHistory(t *testing.T) {
	history := make([]*store.AIMessage, maxHistoryMessages)
	for i := range history {
		role := store.AIMessageRoleUser
		if i%2 == 1 {
			role = store.AIMessageRoleAssistant
		}
		history[i] = &store.AIMessage{Role: role, Content: "msg"}
	}

	messages := buildOpenAIMessages(history, "new")
	// 刚好 maxHistoryMessages 条历史 + 1 条新消息
	assert.Equal(t, maxHistoryMessages+1, len(messages))
}

func TestBuildOpenAIMessages_LastMessageAlwaysUser(t *testing.T) {
	history := []*store.AIMessage{
		{Role: store.AIMessageRoleUser, Content: "q"},
		{Role: store.AIMessageRoleAssistant, Content: "a"},
	}

	messages := buildOpenAIMessages(history, "new question")
	last := messages[len(messages)-1]
	assert.Equal(t, "user", last.Role)
	assert.Equal(t, "new question", last.Content)
}

// ==================== extractChatCompletionResult 测试 ====================

func TestExtractChatCompletionResult_NilResponse(t *testing.T) {
	content, refs, reasoning, usage := extractChatCompletionResult(nil)
	assert.Empty(t, content)
	assert.Empty(t, refs)
	assert.Empty(t, reasoning)
	assert.Empty(t, usage)
}

func TestExtractChatCompletionResult_EmptyChoices(t *testing.T) {
	resp := &ragflow.OpenAIChatResponse{Choices: []ragflow.OpenAIChoice{}}
	content, refs, reasoning, usage := extractChatCompletionResult(resp)
	assert.Empty(t, content)
	assert.Empty(t, refs)
	assert.Empty(t, reasoning)
	assert.Empty(t, usage)
}

func TestExtractChatCompletionResult_NilMessage(t *testing.T) {
	resp := &ragflow.OpenAIChatResponse{
		Choices: []ragflow.OpenAIChoice{{Message: nil}},
	}
	content, refs, reasoning, usage := extractChatCompletionResult(resp)
	assert.Empty(t, content)
	assert.Empty(t, refs)
	assert.Empty(t, reasoning)
	assert.Empty(t, usage)
}

func TestExtractChatCompletionResult_ContentOnly(t *testing.T) {
	resp := &ragflow.OpenAIChatResponse{
		Choices: []ragflow.OpenAIChoice{
			{
				Message: &ragflow.OpenAIResponseMessage{
					Role:    "assistant",
					Content: "Hello, world!",
				},
			},
		},
	}

	content, refs, reasoning, usage := extractChatCompletionResult(resp)
	assert.Equal(t, "Hello, world!", content)
	assert.Empty(t, refs)
	assert.Empty(t, reasoning)
	assert.Empty(t, usage)
}

func TestExtractChatCompletionResult_WithReferences(t *testing.T) {
	resp := &ragflow.OpenAIChatResponse{
		Choices: []ragflow.OpenAIChoice{
			{
				Message: &ragflow.OpenAIResponseMessage{
					Role:    "assistant",
					Content: "Based on your notes...",
					Reference: []ragflow.OpenAIReference{
						{
							ID:           "chunk1",
							Content:      "Go goroutine is lightweight",
							DocumentName: "memo_m_abc123.txt",
							Similarity:   0.85,
						},
					},
				},
			},
		},
	}

	content, refsJSON, _, _ := extractChatCompletionResult(resp)
	assert.Equal(t, "Based on your notes...", content)
	assert.NotEmpty(t, refsJSON)
	assert.Contains(t, refsJSON, "m_abc123")
	assert.Contains(t, refsJSON, "memo")
}

func TestExtractChatCompletionResult_WithUsage(t *testing.T) {
	resp := &ragflow.OpenAIChatResponse{
		Choices: []ragflow.OpenAIChoice{
			{
				Message: &ragflow.OpenAIResponseMessage{
					Role:    "assistant",
					Content: "response",
				},
			},
		},
		Usage: &ragflow.OpenAIUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	_, _, _, usageJSON := extractChatCompletionResult(resp)
	assert.NotEmpty(t, usageJSON)
	assert.Contains(t, usageJSON, "prompt_tokens")
	assert.Contains(t, usageJSON, "150")
}

func TestExtractChatCompletionResult_WithMultipleReferences(t *testing.T) {
	resp := &ragflow.OpenAIChatResponse{
		Choices: []ragflow.OpenAIChoice{
			{
				Message: &ragflow.OpenAIResponseMessage{
					Role:    "assistant",
					Content: "combined answer",
					Reference: []ragflow.OpenAIReference{
						{
							DocumentName: "memo_m_uid1.txt",
							Similarity:   0.9,
							Content:      "memo content",
						},
						{
							DocumentName: "attachment_att_uid2_report.pdf",
							Similarity:   0.7,
							Content:      "attachment content",
						},
						{
							DocumentName: "unknown_doc.docx",
							Similarity:   0.5,
							Content:      "unknown content",
						},
					},
				},
			},
		},
	}

	content, refsJSON, _, _ := extractChatCompletionResult(resp)
	assert.Equal(t, "combined answer", content)
	assert.Contains(t, refsJSON, `"memo"`)
	assert.Contains(t, refsJSON, `"attachment"`)
	assert.Contains(t, refsJSON, `"unknown"`)
}

func TestExtractChatCompletionResult_EmptyReferences(t *testing.T) {
	resp := &ragflow.OpenAIChatResponse{
		Choices: []ragflow.OpenAIChoice{
			{
				Message: &ragflow.OpenAIResponseMessage{
					Role:      "assistant",
					Content:   "no sources",
					Reference: []ragflow.OpenAIReference{},
				},
			},
		},
	}

	_, refsJSON, _, _ := extractChatCompletionResult(resp)
	assert.Empty(t, refsJSON, "空引用列表不应生成 JSON")
}

// ==================== 常量验证 ====================

func TestMaxHistoryConstants(t *testing.T) {
	assert.Equal(t, 10, maxHistoryRounds)
	assert.Equal(t, 20, maxHistoryMessages)
	assert.Equal(t, maxHistoryRounds*2, maxHistoryMessages)
}
