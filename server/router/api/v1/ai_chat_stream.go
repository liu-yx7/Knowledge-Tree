package v1

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

// ==================== SSE 事件类型 ====================

// SSEEventType SSE 事件的 type 字段，前端据此分发处理逻辑
type SSEEventType string

const (
	// SSEEventContent 增量文本 chunk
	SSEEventContent SSEEventType = "content"
	// SSEEventReasoning 思考链增量（DeepSeek 风格，可选）
	SSEEventReasoning SSEEventType = "reasoning"
	// SSEEventDone 流式结束，携带引用、Token 统计、完整消息 ID
	SSEEventDone SSEEventType = "done"
	// SSEEventError 错误事件
	SSEEventError SSEEventType = "error"
)

// SSEEvent 发送给前端的 SSE 事件 JSON 载荷
type SSEEvent struct {
	Type SSEEventType `json:"type"`

	// Content 增量文本（type=content 时非空）
	Content string `json:"content,omitempty"`

	// ReasoningContent 思考链增量（type=reasoning 时非空）
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// 以下字段仅在 type=done 时填充
	MessageID      string `json:"message_id,omitempty"`
	ReferencesJSON string `json:"references_json,omitempty"`
	TokenUsageJSON string `json:"token_usage_json,omitempty"`

	// Error 错误信息（type=error 时非空）
	Error string `json:"error,omitempty"`
}

// ==================== SSE 流式端点 ====================

// RegisterStreamRoutes 注册 SSE 流式端点到 Echo 服务器
// 注意：此方法已弃用。SSE 路由现在在 RegisterGateway 内部注册到 gwGroup 上，
// 确保在 gRPC-Gateway 通配符 Any("/api/v1/*") 之前注册，避免被拦截返回 404。
// 保留此方法仅为向后兼容，实际路由注册在 v1.go 的 RegisterGateway 中。
func (s *APIV1Service) RegisterStreamRoutes(_ *echo.Echo) {
	// No-op: SSE route is now registered inside RegisterGateway's gwGroup
	// to ensure it takes priority over the gRPC-Gateway wildcard route.
}

// handleStreamMessage SSE 流式对话端点
// 编排流程与 SendMessage 一致，区别是用 ChatCompletionStream 逐 chunk 透传
func (s *APIV1Service) handleStreamMessage(c *echo.Context) error {
	ctx := c.Request().Context()

	// ==================== 认证 ====================
	userID, err := s.authenticateHTTPRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// ==================== 解析请求 ====================
	conversationUID := c.Param("conversationId")
	if conversationUID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "conversation_id is required")
	}

	var reqBody struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&reqBody); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if reqBody.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}

	// ==================== 获取 RAGFlow 客户端 ====================
	userClient := s.getUserRAGFlowClient(ctx, userID)
	if userClient == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "AI is not enabled")
	}

	// ==================== 验证对话归属 ====================
	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UID: &conversationUID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
	}
	if len(conversations) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "conversation not found")
	}
	conversation := conversations[0]
	if conversation.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "permission denied")
	}

	// ==================== 获取 AssistantID ====================
	assistantID, err := s.ensureAssistantID(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAGFlow assistant not configured")
	}

	// ==================== 加载历史 + 构建 messages ====================
	orderASC := "ASC"
	limit := maxHistoryMessages
	historyMessages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
		OrderByCreated: &orderASC,
		Limit:          &limit,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load history")
	}
	openaiMessages := buildOpenAIMessages(historyMessages, reqBody.Content)

	// ==================== 保存用户消息 ====================
	userMessage := &store.AIMessage{
		UID:            util.GenUUID(),
		ConversationID: conversation.ID,
		Role:           store.AIMessageRoleUser,
		Content:        reqBody.Content,
	}
	userMessage, err = s.Store.CreateAIMessage(ctx, userMessage)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save user message")
	}

	// ==================== 发起流式调用 ====================
	openaiReq := ragflow.NewOpenAIChatRequest(openaiMessages, true)
	chunkChan, err := userClient.ChatCompletionStream(ctx, assistantID, openaiReq)
	if err != nil {
		slog.Error("RAGFlow 流式 API 调用失败",
			slog.Int("userID", int(userID)),
			slog.String("conversationUID", conversationUID),
			slog.Any("error", err))

		// 保存错误消息
		errMsg := &store.AIMessage{
			UID:            util.GenUUID(),
			ConversationID: conversation.ID,
			Role:           store.AIMessageRoleAssistant,
			Content:        "Sorry, I'm unable to process your request right now. Please try again later.",
		}
		errMsg, _ = s.Store.CreateAIMessage(ctx, errMsg)

		return echo.NewHTTPError(http.StatusBadGateway, "failed to connect to AI service")
	}

	// ==================== SSE 响应头 ====================
	resp := c.Response()
	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no") // 禁用 Nginx 代理缓冲
	resp.WriteHeader(http.StatusOK)

	flusher, canFlush := resp.(http.Flusher)

	// ==================== 流式透传 ====================
	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var referencesJSON string
	var tokenUsageJSON string

	for chunk := range chunkChan {
		// 处理错误 chunk
		if chunk.Error != nil {
			writeSSE(resp, flusher, canFlush, &SSEEvent{
				Type:  SSEEventError,
				Error: chunk.Error.Error(),
			})
			break
		}

		// 透传增量文本
		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)
			writeSSE(resp, flusher, canFlush, &SSEEvent{
				Type:    SSEEventContent,
				Content: chunk.Content,
			})
		}

		// 透传思考链
		if chunk.ReasoningContent != "" {
			reasoningBuilder.WriteString(chunk.ReasoningContent)
			writeSSE(resp, flusher, canFlush, &SSEEvent{
				Type:             SSEEventReasoning,
				ReasoningContent: chunk.ReasoningContent,
			})
		}

		// 最后一个 chunk：提取引用和 Token 统计
		if chunk.Done {
			if len(chunk.References) > 0 {
				parsed := ragflow.ParseReferences(chunk.References)
				if jsonBytes, err := json.Marshal(parsed); err == nil {
					referencesJSON = string(jsonBytes)
				}
			}
			if chunk.Usage != nil {
				if jsonBytes, err := json.Marshal(chunk.Usage); err == nil {
					tokenUsageJSON = string(jsonBytes)
				}
			}
			// 如果 FinalContent 非空，用它替代累积的 content（兜底机制）
			if chunk.FinalContent != "" {
				contentBuilder.Reset()
				contentBuilder.WriteString(chunk.FinalContent)
			}
			break
		}
	}

	// ==================== 保存助手消息 ====================
	assistantMessage := &store.AIMessage{
		UID:              util.GenUUID(),
		ConversationID:   conversation.ID,
		Role:             store.AIMessageRoleAssistant,
		Content:          contentBuilder.String(),
		ReasoningContent: reasoningBuilder.String(),
		ReferencesJSON:   referencesJSON,
		TokenUsageJSON:   tokenUsageJSON,
	}
	assistantMessage, err = s.Store.CreateAIMessage(ctx, assistantMessage)
	if err != nil {
		slog.Error("保存助手消息失败",
			slog.String("conversationUID", conversationUID),
			slog.Any("error", err))
	}

	// ==================== 发送完成事件 ====================
	messageID := ""
	if assistantMessage != nil {
		messageID = assistantMessage.UID
	}
	writeSSE(resp, flusher, canFlush, &SSEEvent{
		Type:           SSEEventDone,
		MessageID:      messageID,
		ReferencesJSON: referencesJSON,
		TokenUsageJSON: tokenUsageJSON,
	})

	// ==================== 自动更新标题 ====================
	s.maybeUpdateConversationTitle(ctx, conversation, reqBody.Content)

	return nil
}

// ==================== SSE 写入辅助 ====================

// writeSSE 将事件 JSON 序列化后写入 SSE 流
// SSE 协议格式: data: {json}\n\n
func writeSSE(resp http.ResponseWriter, flusher http.Flusher, canFlush bool, event *SSEEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	fmt.Fprintf(resp, "data: %s\n\n", data)
	if canFlush {
		flusher.Flush()
	}
}

// ==================== 认证辅助 ====================

// authenticateHTTPRequest 从 Echo HTTP 请求中认证用户
// 支持 Bearer Token（Access Token V2 / PAT）
// 复用 Authenticator 实现，与 gRPC-Gateway / Connect 行为一致
func (s *APIV1Service) authenticateHTTPRequest(c *echo.Context) (int32, error) {
	authenticator := auth.NewAuthenticator(s.Store, s.Secret)

	authHeader := c.Request().Header.Get("Authorization")
	result := authenticator.Authenticate(c.Request().Context(), authHeader)
	if result == nil {
		return 0, fmt.Errorf("no valid credentials")
	}

	if result.Claims != nil {
		return result.Claims.UserID, nil
	}
	if result.User != nil {
		return result.User.ID, nil
	}

	return 0, fmt.Errorf("authentication failed")
}
