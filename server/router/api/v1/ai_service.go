package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/ragflow"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

// ==================== 对话历史限制 ====================

// maxHistoryRounds 发送给 RAGFlow 的最大历史轮数（1 轮 = 1 条 user + 1 条 assistant）
// RAGFlow 内部也会自动截断过长的历史，此限制主要减少网络传输
const maxHistoryRounds = 10

// maxHistoryMessages 最大加载消息条数（maxHistoryRounds * 2）
const maxHistoryMessages = maxHistoryRounds * 2

// ==================== 对话管理 ====================

// CreateConversation creates a new AI conversation.
func (s *APIV1Service) CreateConversation(ctx context.Context, req *v1pb.CreateConversationRequest) (*v1pb.Conversation, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	title := req.Title
	if title == "" {
		title = "New Chat"
	}

	conversation := &store.AIConversation{
		UID:    util.GenUUID(),
		UserID: userID,
		Title:  title,
	}

	created, err := s.Store.CreateAIConversation(ctx, conversation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create conversation: %v", err)
	}

	return convertConversationToProto(created, user.Username), nil
}

// ListConversations lists all conversations for the current user.
func (s *APIV1Service) ListConversations(ctx context.Context, _ *v1pb.ListConversationsRequest) (*v1pb.ListConversationsResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	normalStatus := store.Normal
	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UserID:    &userID,
		RowStatus: &normalStatus,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list conversations: %v", err)
	}

	protoConversations := make([]*v1pb.Conversation, len(conversations))
	for i, c := range conversations {
		protoConversations[i] = convertConversationToProto(c, user.Username)
	}

	return &v1pb.ListConversationsResponse{
		Conversations: protoConversations,
	}, nil
}

// GetConversation gets a specific conversation with messages.
func (s *APIV1Service) GetConversation(ctx context.Context, req *v1pb.GetConversationRequest) (*v1pb.Conversation, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UID: &req.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if len(conversations) == 0 {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	conversation := conversations[0]
	if conversation.UserID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	// 加载消息
	messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list messages: %v", err)
	}

	protoConversation := convertConversationToProto(conversation, user.Username)
	protoConversation.Messages = make([]*v1pb.Message, len(messages))
	for i, m := range messages {
		protoConversation.Messages[i] = convertMessageToProto(m)
	}

	return protoConversation, nil
}

// DeleteConversation deletes a conversation and all its messages.
func (s *APIV1Service) DeleteConversation(ctx context.Context, req *v1pb.DeleteConversationRequest) (*emptypb.Empty, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UID: &req.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if len(conversations) == 0 {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	conversation := conversations[0]
	if conversation.UserID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	// 级联删除消息
	if err := s.Store.DeleteAIMessage(ctx, &store.DeleteAIMessage{ConversationID: &conversation.ID}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete messages: %v", err)
	}

	if err := s.Store.DeleteAIConversation(ctx, &store.DeleteAIConversation{ID: conversation.ID}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete conversation: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateConversation updates conversation metadata.
func (s *APIV1Service) UpdateConversation(ctx context.Context, req *v1pb.UpdateConversationRequest) (*v1pb.Conversation, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UID: &req.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if len(conversations) == 0 {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	conversation := conversations[0]
	if conversation.UserID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	update := &store.UpdateAIConversation{ID: conversation.ID}
	if req.Title != "" {
		update.Title = &req.Title
	}
	now := time.Now().Unix()
	update.UpdatedTs = &now

	if err := s.Store.UpdateAIConversation(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update conversation: %v", err)
	}

	return s.GetConversation(ctx, &v1pb.GetConversationRequest{ConversationId: req.ConversationId})
}

// ==================== 消息发送（P3 OpenAI 兼容 API） ====================

// SendMessage sends a message and gets AI response via RAGFlow OpenAI Compatible API.
// 流程: 加载历史 → 构建 messages → 调用 ChatCompletion → 解析引用 → 保存消息
func (s *APIV1Service) SendMessage(ctx context.Context, req *v1pb.SendMessageRequest) (*v1pb.SendMessageResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if req.Content == "" {
		return nil, status.Errorf(codes.InvalidArgument, "message content cannot be empty")
	}

	// ① 获取 per-user RAGFlow 客户端
	userClient := s.getUserRAGFlowClient(ctx, userID)
	if userClient == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "AI is not enabled (RAGFlow not provisioned for this user)")
	}

	// ② 获取对话
	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UID: &req.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if len(conversations) == 0 {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	conversation := conversations[0]
	if conversation.UserID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	// ③ 获取 AssistantID（从 Provisioner 映射表获取）
	assistantID, err := s.ensureAssistantID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// ④ 加载对话历史，构建 OpenAI messages 数组
	orderASC := "ASC"
	limit := maxHistoryMessages
	historyMessages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
		OrderByCreated: &orderASC,
		Limit:          &limit,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load conversation history: %v", err)
	}

	openaiMessages := buildOpenAIMessages(historyMessages, req.Content)

	// ⑤ 保存用户消息
	userMessage := &store.AIMessage{
		UID:            util.GenUUID(),
		ConversationID: conversation.ID,
		Role:           store.AIMessageRoleUser,
		Content:        req.Content,
	}
	userMessage, err = s.Store.CreateAIMessage(ctx, userMessage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save user message: %v", err)
	}

	// ⑥ 调用 RAGFlow OpenAI 兼容 API（非流式）
	openaiReq := ragflow.NewOpenAIChatRequest(openaiMessages, false)
	chatResp, err := userClient.ChatCompletion(ctx, assistantID, openaiReq)
	if err != nil {
		slog.Error("RAGFlow OpenAI API 调用失败",
			slog.Int("userID", int(userID)),
			slog.String("conversationUID", req.ConversationId),
			slog.Any("error", err))

		// 保存错误消息，让用户知道发生了什么
		errMessage := &store.AIMessage{
			UID:            util.GenUUID(),
			ConversationID: conversation.ID,
			Role:           store.AIMessageRoleAssistant,
			Content:        "Sorry, I'm unable to process your request right now. Please try again later.",
		}
		errMessage, _ = s.Store.CreateAIMessage(ctx, errMessage)

		return &v1pb.SendMessageResponse{
			UserMessage:      convertMessageToProto(userMessage),
			AssistantMessage: convertMessageToProto(errMessage),
		}, nil
	}

	// ⑦ 提取响应内容和引用
	assistantContent, referencesJSON, reasoningContent, tokenUsageJSON := extractChatCompletionResult(chatResp)

	// ⑧ 保存助手消息
	assistantMessage := &store.AIMessage{
		UID:              util.GenUUID(),
		ConversationID:   conversation.ID,
		Role:             store.AIMessageRoleAssistant,
		Content:          assistantContent,
		ReasoningContent: reasoningContent,
		ReferencesJSON:   referencesJSON,
		TokenUsageJSON:   tokenUsageJSON,
	}
	assistantMessage, err = s.Store.CreateAIMessage(ctx, assistantMessage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save assistant message: %v", err)
	}

	// ⑨ 首次对话时自动更新标题（取用户消息前 50 字符）
	s.maybeUpdateConversationTitle(ctx, conversation, req.Content)

	return &v1pb.SendMessageResponse{
		UserMessage:      convertMessageToProto(userMessage),
		AssistantMessage: convertMessageToProto(assistantMessage),
	}, nil
}

// ==================== SendMessage 辅助函数 ====================

// ensureAssistantID 确保用户的 RAGFlow Chat Assistant 就绪并返回 AssistantID
// 优先通过 Provisioner 触发完整资源配置（认证 + Dataset + Assistant），
// 避免 getUserRAGFlowClient 只配置认证而 AssistantID 仍为空的"半初始化"问题。
// 降级路径：被动查询 ragflow_user_mapping 表。
func (s *APIV1Service) ensureAssistantID(ctx context.Context, userID int32) (string, error) {
	// 优先路径：通过 Provisioner 确保全部资源就绪（认证 + Dataset + Assistant）
	if s.RAGFlowProvisioner != nil {
		user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
		if err != nil || user == nil {
			return "", status.Errorf(codes.Internal, "failed to get user info: %v", err)
		}

		// EnsureUserResources 会依次调用:
		// GetClientForUser（认证） → ensureDataset → ensureAssistant
		// 确保 AssistantID 被写入 mapping 表
		if _, _, err := s.RAGFlowProvisioner.EnsureUserResources(ctx, userID, user.Username); err != nil {
			slog.Warn("ensureAssistantID: Provisioner 资源配置失败",
				slog.Int("userID", int(userID)),
				slog.Any("error", err))
			// 降级到被动查询
		}
	}

	// 从 mapping 表读取 AssistantID
	mapping, err := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{UserID: &userID})
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to get RAGFlow user mapping: %v", err)
	}
	if mapping == nil || mapping.AssistantID == "" {
		return "", status.Errorf(codes.FailedPrecondition, "RAGFlow assistant not configured for this user")
	}
	return mapping.AssistantID, nil
}

// buildOpenAIMessages 从历史消息和新消息构建 OpenAI messages 数组
// 规则：
//   - 不发送 system 消息（RAGFlow Chat Assistant 自带 prompt 配置）
//   - 按时间顺序排列历史 user/assistant 消息
//   - 最后追加本次用户消息
//   - 如果历史超过 maxHistoryMessages，只取最近的 N 条
func buildOpenAIMessages(history []*store.AIMessage, newContent string) []ragflow.OpenAIMessage {
	// 预分配: 历史消息 + 新消息
	messages := make([]ragflow.OpenAIMessage, 0, len(history)+1)

	// 如果历史消息过多，只取最近的 maxHistoryMessages 条
	start := 0
	if len(history) > maxHistoryMessages {
		start = len(history) - maxHistoryMessages
	}

	for _, msg := range history[start:] {
		role := "user"
		if msg.Role == store.AIMessageRoleAssistant {
			role = "assistant"
		}
		messages = append(messages, ragflow.OpenAIMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// 追加本次用户消息
	messages = append(messages, ragflow.OpenAIMessage{
		Role:    "user",
		Content: newContent,
	})

	return messages
}

// extractChatCompletionResult 从非流式 ChatCompletion 响应中提取内容、引用、思考链、Token 使用
func extractChatCompletionResult(resp *ragflow.OpenAIChatResponse) (content, referencesJSON, reasoningContent, tokenUsageJSON string) {
	if resp == nil || len(resp.Choices) == 0 {
		return "", "", "", ""
	}

	choice := resp.Choices[0]
	if choice.Message == nil {
		return "", "", "", ""
	}

	content = choice.Message.Content

	// 解析引用信息 → ParsedReference JSON
	if len(choice.Message.Reference) > 0 {
		parsed := ragflow.ParseReferences(choice.Message.Reference)
		if jsonBytes, err := json.Marshal(parsed); err == nil {
			referencesJSON = string(jsonBytes)
		}
	}

	// Token 使用统计
	if resp.Usage != nil {
		if jsonBytes, err := json.Marshal(resp.Usage); err == nil {
			tokenUsageJSON = string(jsonBytes)
		}
	}

	// 非流式响应中 reasoning_content 不在标准 OpenAI 格式中
	// 如果后续 RAGFlow 扩展支持，可在此处提取

	return content, referencesJSON, reasoningContent, tokenUsageJSON
}

// maybeUpdateConversationTitle 首次对话时自动设置对话标题
func (s *APIV1Service) maybeUpdateConversationTitle(ctx context.Context, conversation *store.AIConversation, userContent string) {
	messages, _ := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	// 只在第一轮对话（2 条消息：user + assistant）时自动设置标题
	if len(messages) > 2 {
		return
	}

	title := userContent
	runes := []rune(title)
	if len(runes) > 50 {
		title = string(runes[:50]) + "..."
	}
	now := time.Now().Unix()
	_ = s.Store.UpdateAIConversation(ctx, &store.UpdateAIConversation{
		ID:        conversation.ID,
		Title:     &title,
		UpdatedTs: &now,
	})
}

// ==================== 消息查询 ====================

// ListMessages lists all messages in a conversation.
func (s *APIV1Service) ListMessages(ctx context.Context, req *v1pb.ListMessagesRequest) (*v1pb.ListMessagesResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UID: &req.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if len(conversations) == 0 {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	conversation := conversations[0]
	if conversation.UserID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list messages: %v", err)
	}

	protoMessages := make([]*v1pb.Message, len(messages))
	for i, m := range messages {
		protoMessages[i] = convertMessageToProto(m)
	}

	return &v1pb.ListMessagesResponse{
		Messages: protoMessages,
	}, nil
}

// ==================== AI 配置查询 ====================

// GetAIConfig returns AI configuration (RAGFlow mode).
func (s *APIV1Service) GetAIConfig(ctx context.Context, _ *v1pb.GetAIConfigRequest) (*v1pb.GetAIConfigResponse, error) {
	if s.RAGFlowClient == nil {
		return &v1pb.GetAIConfigResponse{
			Enabled: false,
		}, nil
	}

	protoProviders := []*v1pb.AIProvider{
		{
			Name:        "ragflow",
			DisplayName: "RAGFlow",
			Models:      []string{"default"},
		},
	}

	return &v1pb.GetAIConfigResponse{
		Enabled:         true,
		Providers:       protoProviders,
		DefaultProvider: "ragflow",
		DefaultModel:    "default",
	}, nil
}

// ==================== 转换辅助函数 ====================

func convertConversationToProto(c *store.AIConversation, username string) *v1pb.Conversation {
	return &v1pb.Conversation{
		Id:         c.UID,
		User:       fmt.Sprintf("users/%s", username),
		Title:      c.Title,
		CreateTime: timestamppb.New(time.Unix(c.CreatedTs, 0)),
		UpdateTime: timestamppb.New(time.Unix(c.UpdatedTs, 0)),
	}
}

func convertMessageToProto(m *store.AIMessage) *v1pb.Message {
	role := v1pb.MessageRole_MESSAGE_ROLE_UNSPECIFIED
	switch m.Role {
	case store.AIMessageRoleUser:
		role = v1pb.MessageRole_USER
	case store.AIMessageRoleAssistant:
		role = v1pb.MessageRole_ASSISTANT
	}

	return &v1pb.Message{
		Id:               m.UID,
		Role:             role,
		Content:          m.Content,
		ReasoningContent: m.ReasoningContent,
		ReferencesJson:   m.ReferencesJSON,
		CreateTime:       timestamppb.New(time.Unix(m.CreatedTs, 0)),
	}
}
