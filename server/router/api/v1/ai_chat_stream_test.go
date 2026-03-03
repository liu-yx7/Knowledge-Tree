package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== SSEEvent 类型与序列化测试 ====================

func TestSSEEventType_Constants(t *testing.T) {
	assert.Equal(t, SSEEventType("content"), SSEEventContent)
	assert.Equal(t, SSEEventType("reasoning"), SSEEventReasoning)
	assert.Equal(t, SSEEventType("done"), SSEEventDone)
	assert.Equal(t, SSEEventType("error"), SSEEventError)
}

func TestSSEEvent_ContentJSON(t *testing.T) {
	event := &SSEEvent{
		Type:    SSEEventContent,
		Content: "Hello",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "content", parsed["type"])
	assert.Equal(t, "Hello", parsed["content"])
	// omitempty 字段不应出现
	assert.NotContains(t, parsed, "error")
	assert.NotContains(t, parsed, "message_id")
	assert.NotContains(t, parsed, "references_json")
	assert.NotContains(t, parsed, "token_usage_json")
	assert.NotContains(t, parsed, "reasoning_content")
}

func TestSSEEvent_ReasoningJSON(t *testing.T) {
	event := &SSEEvent{
		Type:             SSEEventReasoning,
		ReasoningContent: "thinking step 1...",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "reasoning", parsed["type"])
	assert.Equal(t, "thinking step 1...", parsed["reasoning_content"])
	assert.NotContains(t, parsed, "content")
}

func TestSSEEvent_DoneJSON(t *testing.T) {
	event := &SSEEvent{
		Type:           SSEEventDone,
		MessageID:      "msg_abc123",
		ReferencesJSON: `[{"type":"memo","memo_uid":"m_xyz"}]`,
		TokenUsageJSON: `{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}`,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "done", parsed["type"])
	assert.Equal(t, "msg_abc123", parsed["message_id"])
	assert.Contains(t, parsed["references_json"], "memo")
	assert.Contains(t, parsed["token_usage_json"], "150")
	// content 应该被 omit
	assert.NotContains(t, parsed, "content")
}

func TestSSEEvent_ErrorJSON(t *testing.T) {
	event := &SSEEvent{
		Type:  SSEEventError,
		Error: "connection timeout",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "error", parsed["type"])
	assert.Equal(t, "connection timeout", parsed["error"])
	assert.NotContains(t, parsed, "content")
}

func TestSSEEvent_DoneWithEmptyOptionals(t *testing.T) {
	// done 事件没有引用和 Token 统计时，这些字段应被 omit
	event := &SSEEvent{
		Type:      SSEEventDone,
		MessageID: "msg_123",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "done", parsed["type"])
	assert.Equal(t, "msg_123", parsed["message_id"])
	assert.NotContains(t, parsed, "references_json")
	assert.NotContains(t, parsed, "token_usage_json")
}

func TestSSEEvent_Roundtrip(t *testing.T) {
	// 验证序列化/反序列化往返一致性
	original := &SSEEvent{
		Type:           SSEEventDone,
		MessageID:      "msg_roundtrip",
		ReferencesJSON: `[{"type":"memo"}]`,
		TokenUsageJSON: `{"total_tokens":200}`,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored SSEEvent
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, original.Type, restored.Type)
	assert.Equal(t, original.MessageID, restored.MessageID)
	assert.Equal(t, original.ReferencesJSON, restored.ReferencesJSON)
	assert.Equal(t, original.TokenUsageJSON, restored.TokenUsageJSON)
}

func TestSSEEvent_ContentWithSpecialChars(t *testing.T) {
	// 验证中文、换行、特殊字符的正确序列化
	event := &SSEEvent{
		Type:    SSEEventContent,
		Content: "你好世界\n\"引号\" & <标签>",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var restored SSEEvent
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, event.Content, restored.Content)
}

func TestSSEEvent_EmptyContent(t *testing.T) {
	// 空 content 应被 omit
	event := &SSEEvent{
		Type:    SSEEventContent,
		Content: "",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "content", parsed["type"])
	assert.NotContains(t, parsed, "content", "空 content 应被 omitempty")
}
