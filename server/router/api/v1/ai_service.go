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
	"github.com/usememos/memos/plugin/ragflow"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

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

	// RAGFlow 模式下，provider 和 model 由 RAGFlow 服务管理
	model := req.Model
	provider := "ragflow"

	conversation := &store.AIConversation{
		UID:      util.GenUUID(),
		UserID:   userID,
		Title:    title,
		Model:    model,
		Provider: provider,
	}

	created, err := s.Store.CreateAIConversation(ctx, conversation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create conversation: %v", err)
	}

	// 如果 RAGFlow 客户端可用，在 RAGFlow 中创建对应的会话
	if s.RAGFlowClient != nil {
		assistantID := s.RAGFlowClient.GetConfig().AssistantID
		if assistantID != "" {
			session, err := s.RAGFlowClient.CreateSession(ctx, assistantID, title)
			if err == nil {
				// 存储 RAGFlow session ID 到 conversation（可选：扩展数据库字段）
				// 目前使用 UID 作为映射标识
				_ = session // TODO: 可以将 session.ID 存储到扩展字段
			}
		}
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

	// Fetch messages
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
	if req.Model != "" {
		update.Model = &req.Model
	}
	if req.Provider != "" {
		update.Provider = &req.Provider
	}
	now := time.Now().Unix()
	update.UpdatedTs = &now

	if err := s.Store.UpdateAIConversation(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update conversation: %v", err)
	}

	// Fetch updated conversation
	return s.GetConversation(ctx, &v1pb.GetConversationRequest{ConversationId: req.ConversationId})
}

// SendMessage sends a message and gets AI response via RAGFlow.
func (s *APIV1Service) SendMessage(ctx context.Context, req *v1pb.SendMessageRequest) (*v1pb.SendMessageResponse, error) {
	userID := auth.GetUserID(ctx)
	if userID == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if s.RAGFlowClient == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "AI is not enabled (RAGFlow not configured)")
	}

	// Get conversation
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

	// Save user message
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

	// 调用 RAGFlow Chat API
	assistantID := s.RAGFlowClient.GetConfig().AssistantID
	if assistantID == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "RAGFlow assistant not configured")
	}

	// 使用会话 UID 作为 RAGFlow session ID（简化映射）
	chatReq := &ragflow.ChatRequest{
		SessionID: conversation.UID,
		Question:  req.Content,
		Stream:    false,
	}

	chatResp, err := s.RAGFlowClient.Chat(ctx, assistantID, chatReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get AI response from RAGFlow: %v", err)
	}

	// Save assistant message
	assistantMessage := &store.AIMessage{
		UID:            util.GenUUID(),
		ConversationID: conversation.ID,
		Role:           store.AIMessageRoleAssistant,
		Content:        chatResp.Answer,
		TokenCount:     0, // RAGFlow 不直接返回 token count
	}
	assistantMessage, err = s.Store.CreateAIMessage(ctx, assistantMessage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save assistant message: %v", err)
	}

	// Fetch existing messages count for title update
	messages, _ := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})

	// Update conversation title if it's the first exchange
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

// GetAIConfig returns AI configuration (RAGFlow mode).
func (s *APIV1Service) GetAIConfig(ctx context.Context, _ *v1pb.GetAIConfigRequest) (*v1pb.GetAIConfigResponse, error) {
	if s.RAGFlowClient == nil {
		return &v1pb.GetAIConfigResponse{
			Enabled: false,
		}, nil
	}

	// RAGFlow 模式下，只有一个 provider
	protoProviders := []*v1pb.AIProvider{
		{
			Name:        "ragflow",
			DisplayName: "RAGFlow",
			Models:      []string{"default"}, // RAGFlow 内部管理模型
		},
	}

	return &v1pb.GetAIConfigResponse{
		Enabled:         true,
		Providers:       protoProviders,
		DefaultProvider: "ragflow",
		DefaultModel:    "default",
	}, nil
}

// Helper functions

func convertConversationToProto(c *store.AIConversation, username string) *v1pb.Conversation {
	return &v1pb.Conversation{
		Id:         c.UID,
		User:       fmt.Sprintf("users/%s", username),
		Title:      c.Title,
		Model:      c.Model,
		Provider:   c.Provider,
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
	case store.AIMessageRoleSystem:
		role = v1pb.MessageRole_SYSTEM
	}

	return &v1pb.Message{
		Id:         m.UID,
		Role:       role,
		Content:    m.Content,
		CreateTime: timestamppb.New(time.Unix(m.CreatedTs, 0)),
		TokenCount: m.TokenCount,
	}
}
