package v1

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/internal/util"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

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

// ==================== 消息发送（P3 临时实现，待 OpenAI 兼容客户端完成后替换） ====================

// SendMessage sends a message and gets AI response.
// TODO(P3): 当前为占位实现，完整版将使用 OpenAI 兼容 API + 流式 SSE
func (s *APIV1Service) SendMessage(ctx context.Context, req *v1pb.SendMessageRequest) (*v1pb.SendMessageResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	// 获取 per-user RAGFlow 客户端
	userClient := s.getUserRAGFlowClient(ctx, userID)
	if userClient == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "AI is not enabled (RAGFlow not provisioned for this user)")
	}

	// 获取对话
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

	// 保存用户消息
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

	// 获取 AssistantID（从 Provisioner 映射表获取）
	assistantID := ""
	mapping, _ := s.Store.GetRAGFlowUserMapping(ctx, &store.FindRAGFlowUserMapping{UserID: &userID})
	if mapping != nil {
		assistantID = mapping.AssistantID
	}
	if assistantID == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "RAGFlow assistant not configured for this user")
	}

	// TODO(P3): 替换为 OpenAI 兼容 API 调用
	// 当前临时使用 SDK API 的 Retrieve 做简单问答
	// 完整版将：
	//   1. 从 DB 加载最近 N 轮历史
	//   2. 构建 OpenAI messages 数组
	//   3. 调用 ChatCompletionStream (plugin/ragflow/openai.go)
	//   4. 流式转发 + 提取引用
	assistantContent := "AI service is being upgraded to RAGFlow OpenAI compatible API. Please try again later."

	// 保存助手消息
	assistantMessage := &store.AIMessage{
		UID:            util.GenUUID(),
		ConversationID: conversation.ID,
		Role:           store.AIMessageRoleAssistant,
		Content:        assistantContent,
	}
	assistantMessage, err = s.Store.CreateAIMessage(ctx, assistantMessage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save assistant message: %v", err)
	}

	// 首次对话时自动更新标题
	messages, _ := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	if len(messages) <= 2 {
		title := req.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		now := time.Now().Unix()
		_ = s.Store.UpdateAIConversation(ctx, &store.UpdateAIConversation{
			ID:        conversation.ID,
			Title:     &title,
			UpdatedTs: &now,
		})
	}

	return &v1pb.SendMessageResponse{
		UserMessage:      convertMessageToProto(userMessage),
		AssistantMessage: convertMessageToProto(assistantMessage),
	}, nil
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
