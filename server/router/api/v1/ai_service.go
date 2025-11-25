package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/internal/ai"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) CreateConversation(ctx context.Context, request *v1pb.CreateConversationRequest) (*v1pb.Conversation, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// Validate provider and model
	if _, err := s.AIManager.GetProvider(request.LlmProvider); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "provider %s not found or not configured", request.LlmProvider)
	}

	create := &store.AIConversation{
		Name:         request.Name,
		CreatorID:    user.ID,
		LLMProvider:  request.LlmProvider,
		LLMModel:     request.LlmModel,
		SystemPrompt: request.SystemPrompt,
	}

	conversation, err := s.Store.CreateAIConversation(ctx, create)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create conversation: %v", err)
	}

	return convertConversationFromStore(conversation, 0), nil
}

func (s *APIV1Service) ListConversations(ctx context.Context, request *v1pb.ListConversationsRequest) (*v1pb.ListConversationsResponse, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	find := &store.FindAIConversation{
		CreatorID: &user.ID,
	}

	// Handle pagination
	var limit, offset int
	if request.PageSize > 0 {
		limit = int(request.PageSize)
	} else {
		limit = 50 // default
	}
	if request.PageToken != "" {
		var pageToken v1pb.PageToken
		if err := unmarshalPageToken(request.PageToken, &pageToken); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page token: %v", err)
		}
		offset = int(pageToken.Offset)
	}

	find.Limit = &limit
	find.Offset = &offset

	conversations, err := s.Store.ListAIConversations(ctx, find)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list conversations: %v", err)
	}

	// Get message counts for each conversation
	conversationMessages := make([]*v1pb.Conversation, 0, len(conversations))
	for _, conv := range conversations {
		messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
			ConversationID: &conv.ID,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get message count: %v", err)
		}
		conversationMessages = append(conversationMessages, convertConversationFromStore(conv, int32(len(messages))))
	}

	return &v1pb.ListConversationsResponse{
		Conversations: conversationMessages,
	}, nil
}

func (s *APIV1Service) GetConversation(ctx context.Context, request *v1pb.GetConversationRequest) (*v1pb.Conversation, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	conversation, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &request.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if conversation == nil {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	// Verify ownership
	if conversation.CreatorID != user.ID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get message count: %v", err)
	}

	return convertConversationFromStore(conversation, int32(len(messages))), nil
}

func (s *APIV1Service) UpdateConversation(ctx context.Context, request *v1pb.UpdateConversationRequest) (*v1pb.Conversation, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	conversation, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &request.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if conversation == nil {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	// Verify ownership
	if conversation.CreatorID != user.ID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	now := time.Now().Unix()
	update := &store.UpdateAIConversation{
		ID:        request.ConversationId,
		UpdatedTs: &now,
	}
	if request.Name != "" {
		update.Name = &request.Name
	}
	if request.SystemPrompt != "" {
		update.SystemPrompt = &request.SystemPrompt
	}

	if err := s.Store.UpdateAIConversation(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update conversation: %v", err)
	}

	// Get updated conversation
	conversation, err = s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &request.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}

	messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get message count: %v", err)
	}

	return convertConversationFromStore(conversation, int32(len(messages))), nil
}

func (s *APIV1Service) DeleteConversation(ctx context.Context, request *v1pb.DeleteConversationRequest) (*emptypb.Empty, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	conversation, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &request.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if conversation == nil {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}

	// Verify ownership
	if conversation.CreatorID != user.ID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	// Delete all messages in the conversation
	messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list messages: %v", err)
	}
	for _, msg := range messages {
		if err := s.Store.DeleteAIMessage(ctx, &store.DeleteAIMessage{ID: msg.ID}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to delete message: %v", err)
		}
	}

	// Delete the conversation
	if err := s.Store.DeleteAIConversation(ctx, &store.DeleteAIConversation{ID: conversation.ID}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete conversation: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) SendMessage(request *v1pb.SendMessageRequest, stream v1pb.AIService_SendMessageServer) error {
	ctx := stream.Context()

	fmt.Printf("=== SendMessage called ===\n")
	fmt.Printf("ConversationID: %d\n", request.ConversationId)
	fmt.Printf("Content length: %d\n", len(request.Content))

	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		fmt.Printf("ERROR: Failed to get user: %v\n", err)
		return status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		fmt.Printf("ERROR: User not authenticated\n")
		return status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	fmt.Printf("User authenticated: %d (%s)\n", user.ID, user.Username)

	// Get conversation
	conversation, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &request.ConversationId,
	})
	if err != nil {
		fmt.Printf("ERROR: Failed to get conversation: %v\n", err)
		return status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if conversation == nil {
		fmt.Printf("ERROR: Conversation not found\n")
		return status.Errorf(codes.NotFound, "conversation not found")
	}

	fmt.Printf("Conversation found: %s (provider=%s, model=%s)\n", conversation.Name, conversation.LLMProvider, conversation.LLMModel)

	// Verify ownership
	if conversation.CreatorID != user.ID {
		fmt.Printf("ERROR: Permission denied (creator=%d, user=%d)\n", conversation.CreatorID, user.ID)
		return status.Errorf(codes.PermissionDenied, "permission denied")
	}

	// Save user message
	userMessage := &store.AIMessage{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        request.Content,
		Tokens:         0,
	}
	if _, err := s.Store.CreateAIMessage(ctx, userMessage); err != nil {
		fmt.Printf("ERROR: Failed to save user message: %v\n", err)
		return status.Errorf(codes.Internal, "failed to save user message: %v", err)
	}

	fmt.Printf("User message saved\n")

	// Get conversation history
	messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversation.ID,
	})
	if err != nil {
		fmt.Printf("ERROR: Failed to get conversation history: %v\n", err)
		return status.Errorf(codes.Internal, "failed to get conversation history: %v", err)
	}

	fmt.Printf("Retrieved %d messages from history\n", len(messages))

	// Prepare AI request
	aiMessages := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		aiMessages = append(aiMessages, ai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	chatRequest := ai.ChatRequest{
		Messages:     aiMessages,
		Model:        conversation.LLMModel,
		SystemPrompt: conversation.SystemPrompt,
		Temperature:  0.7,
		MaxTokens:    4096,
	}

	fmt.Printf("Calling AI provider: %s with %d messages\n", conversation.LLMProvider, len(aiMessages))

	// Stream response from LLM
	contentCh, errCh := s.AIManager.StreamMessage(ctx, conversation.LLMProvider, chatRequest)

	fmt.Printf("Streaming started, waiting for response...\n")

	var fullContent strings.Builder
	chunkCount := 0
	for {
		select {
		case content, ok := <-contentCh:
			if !ok {
				fmt.Printf("Stream finished. Total chunks: %d, Total content length: %d\n", chunkCount, fullContent.Len())
				// Stream finished, save assistant message
				assistantMessage := &store.AIMessage{
					ConversationID: conversation.ID,
					Role:           "assistant",
					Content:        fullContent.String(),
					Tokens:         0, // TODO: implement token counting
				}
				savedMsg, err := s.Store.CreateAIMessage(ctx, assistantMessage)
				if err != nil {
					fmt.Printf("ERROR: Failed to save assistant message: %v\n", err)
					return status.Errorf(codes.Internal, "failed to save assistant message: %v", err)
				}

				// Send final chunk
				if err := stream.Send(&v1pb.MessageChunk{
					Content: "",
					IsFinal: true,
					Message: convertMessageFromStore(savedMsg),
				}); err != nil {
					fmt.Printf("ERROR: Failed to send final chunk: %v\n", err)
					return status.Errorf(codes.Internal, "failed to send final chunk: %v", err)
				}
				fmt.Printf("=== SendMessage completed successfully ===\n")
				return nil
			}
			chunkCount++
			fullContent.WriteString(content)
			if chunkCount <= 5 || chunkCount%10 == 0 {
				fmt.Printf("Chunk %d received (length: %d)\n", chunkCount, len(content))
			}
			// Send streaming chunk
			if err := stream.Send(&v1pb.MessageChunk{
				Content: content,
				IsFinal: false,
			}); err != nil {
				fmt.Printf("ERROR: Failed to send chunk: %v\n", err)
				return status.Errorf(codes.Internal, "failed to send chunk: %v", err)
			}
		case err := <-errCh:
			if err != nil {
				fmt.Printf("ERROR: LLM error: %v\n", err)
				return status.Errorf(codes.Internal, "LLM error: %v", err)
			}
		case <-ctx.Done():
			fmt.Printf("ERROR: Request canceled\n")
			return status.Errorf(codes.Canceled, "request canceled")
		}
	}
}

func (s *APIV1Service) ListMessages(ctx context.Context, request *v1pb.ListMessagesRequest) (*v1pb.ListMessagesResponse, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// Verify conversation ownership
	conversation, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		ID: &request.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if conversation == nil {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}
	if conversation.CreatorID != user.ID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	messages, err := s.Store.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &request.ConversationId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list messages: %v", err)
	}

	messageList := make([]*v1pb.Message, 0, len(messages))
	for _, msg := range messages {
		messageList = append(messageList, convertMessageFromStore(msg))
	}

	return &v1pb.ListMessagesResponse{
		Messages: messageList,
	}, nil
}

func (s *APIV1Service) ListProviders(ctx context.Context, request *v1pb.ListProvidersRequest) (*v1pb.ListProvidersResponse, error) {
	providers := s.AIManager.ListProviders()

	fmt.Printf("ListProviders called - Found %d providers in AIManager\n", len(providers))

	providerList := make([]*v1pb.Provider, 0, len(providers))
	for _, provider := range providers {
		fmt.Printf("Processing provider: %s\n", provider.Name())

		// Check if provider is configured
		config, err := s.Store.GetAIProviderConfig(ctx, &store.FindAIProviderConfig{
			Name: stringPtr(provider.Name()),
		})

		if err != nil {
			fmt.Printf("  Error getting config: %v\n", err)
		} else if config != nil {
			fmt.Printf("  Config found - enabled=%v, apiKey length=%d\n", config.Enabled, len(config.APIKey))
		} else {
			fmt.Printf("  Config is nil\n")
		}

		configured := config != nil && config.APIKey != ""
		enabled := config != nil && config.Enabled

		fmt.Printf("  Final values - enabled=%v, configured=%v\n", enabled, configured)

		providerList = append(providerList, &v1pb.Provider{
			Name:            provider.Name(),
			DisplayName:     getProviderDisplayName(provider.Name()),
			ApiEndpoint:     getProviderEndpoint(config),
			AvailableModels: provider.GetModels(),
			Enabled:         enabled,
			Configured:      configured,
		})
	}

	fmt.Printf("Returning %d providers to frontend\n", len(providerList))
	return &v1pb.ListProvidersResponse{
		Providers: providerList,
	}, nil
}

func (s *APIV1Service) ConfigureProvider(ctx context.Context, request *v1pb.ConfigureProviderRequest) (*v1pb.Provider, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// Only admin can configure providers
	if user.Role != store.RoleAdmin && user.Role != store.RoleHost {
		return nil, status.Errorf(codes.PermissionDenied, "only admin can configure providers")
	}

	// Check if provider exists
	existing, err := s.Store.GetAIProviderConfig(ctx, &store.FindAIProviderConfig{
		Name: &request.Name,
	})

	if existing != nil {
		// Update existing config
		now := time.Now().Unix()
		update := &store.UpdateAIProviderConfig{
			ID:        existing.ID,
			UpdatedTs: &now,
		}
		if request.DisplayName != "" {
			update.DisplayName = &request.DisplayName
		}
		if request.ApiKey != "" {
			update.APIKey = &request.ApiKey
		}
		if request.ApiEndpoint != "" {
			update.APIEndpoint = &request.ApiEndpoint
		}
		if request.Config != "" {
			update.Config = &request.Config
		}
		update.Enabled = &request.Enabled

		if err := s.Store.UpdateAIProviderConfig(ctx, update); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update provider config: %v", err)
		}
	} else {
		// Create new config
		create := &store.AIProviderConfig{
			Name:        request.Name,
			DisplayName: request.DisplayName,
			APIKey:      request.ApiKey,
			APIEndpoint: request.ApiEndpoint,
			Config:      request.Config,
			Enabled:     request.Enabled,
		}
		if _, err := s.Store.CreateAIProviderConfig(ctx, create); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create provider config: %v", err)
		}
	}

	// Re-initialize providers
	if err := s.initializeAIProviders(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize providers: %v", err)
	}

	return &v1pb.Provider{
		Name:        request.Name,
		DisplayName: request.DisplayName,
		ApiEndpoint: request.ApiEndpoint,
		Enabled:     request.Enabled,
		Configured:  request.ApiKey != "",
	}, nil
}

// Helper functions

func convertConversationFromStore(conversation *store.AIConversation, messageCount int32) *v1pb.Conversation {
	return &v1pb.Conversation{
		Id:           conversation.ID,
		Name:         conversation.Name,
		CreatorId:    conversation.CreatorID,
		LlmProvider:  conversation.LLMProvider,
		LlmModel:     conversation.LLMModel,
		SystemPrompt: conversation.SystemPrompt,
		CreatedTime:  timestamppb.New(time.Unix(conversation.CreatedTs, 0)),
		UpdatedTime:  timestamppb.New(time.Unix(conversation.UpdatedTs, 0)),
		MessageCount: messageCount,
	}
}

func convertMessageFromStore(message *store.AIMessage) *v1pb.Message {
	return &v1pb.Message{
		Id:             message.ID,
		ConversationId: message.ConversationID,
		Role:           message.Role,
		Content:        message.Content,
		Tokens:         message.Tokens,
		CreatedTime:    timestamppb.New(time.Unix(message.CreatedTs, 0)),
	}
}

func getProviderDisplayName(name string) string {
	switch name {
	case "openai":
		return "OpenAI"
	case "deepseek":
		return "Deepseek"
	default:
		return name
	}
}

func getProviderEndpoint(config *store.AIProviderConfig) string {
	if config != nil && config.APIEndpoint != "" {
		return config.APIEndpoint
	}
	return ""
}

func stringPtr(s string) *string {
	return &s
}
