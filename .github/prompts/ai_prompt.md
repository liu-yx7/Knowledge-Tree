# AI Chat Feature Development Prompt

## Overview

Add a ChatGPT-like AI chat interface to the Memos knowledge management platform. This feature will enable users to interact with AI assistants, ask questions about their memos, get writing suggestions, and have general conversations, all while maintaining conversation history and supporting multiple LLM providers.

## Project Context

**Architecture:**

- **Backend:** Go with gRPC services, gRPC-Gateway for REST API
- **Frontend:** React 18 + TypeScript + Vite, MobX for state management, Tailwind CSS + Radix UI
- **Database:** Multi-driver support (SQLite, MySQL, PostgreSQL)
- **API Pattern:** Protocol Buffers (proto/api/v1/) for service definitions

**Key Conventions:**

- Follow existing patterns in `proto/api/v1/` for service definitions
- Backend services in `server/router/api/v1/`
- Frontend pages in `web/src/pages/`, components in `web/src/components/`
- Store layer follows repository pattern with driver interface
- Navigation items defined in `web/src/components/Navigation.tsx`

---

## Step-by-Step Implementation Plan

### Phase 1: Database Schema & Store Layer

#### 1.1 Define Database Models

Create database schema for AI chat conversations and messages:

**Tables to create:**

1. **`ai_conversation`** - Stores conversation sessions

   - `id` (INTEGER PRIMARY KEY)
   - `name` (TEXT) - Conversation title
   - `creator_id` (INTEGER) - Foreign key to user
   - `llm_provider` (TEXT) - Provider name (e.g., "openai", "deepseek")
   - `llm_model` (TEXT) - Model name (e.g., "gpt-4", "deepseek-chat")
   - `system_prompt` (TEXT) - Optional system prompt
   - `created_ts` (BIGINT) - Unix timestamp
   - `updated_ts` (BIGINT) - Unix timestamp

2. **`ai_message`** - Stores individual messages in conversations

   - `id` (INTEGER PRIMARY KEY)
   - `conversation_id` (INTEGER) - Foreign key to ai_conversation
   - `role` (TEXT) - "user" or "assistant"
   - `content` (TEXT) - Message content
   - `tokens` (INTEGER) - Token count (optional)
   - `created_ts` (BIGINT) - Unix timestamp

3. **`ai_provider_config`** - Stores LLM provider API configurations
   - `id` (INTEGER PRIMARY KEY)
   - `name` (TEXT UNIQUE) - Provider identifier
   - `display_name` (TEXT) - Human-readable name
   - `api_key` (TEXT) - Encrypted API key
   - `api_endpoint` (TEXT) - Custom endpoint URL
   - `config` (TEXT) - JSON config for additional settings
   - `enabled` (BOOLEAN) - Whether provider is active
   - `created_ts` (BIGINT)
   - `updated_ts` (BIGINT)

**Files to create:**

1. `store/ai_conversation.go` - CRUD operations for conversations
2. `store/ai_message.go` - CRUD operations for messages
3. `store/ai_provider_config.go` - CRUD operations for provider configs
4. `store/db/sqlite/migration/prod_YYYYMMDD_ai_chat.sql` - SQLite migration
5. `store/db/mysql/migration/prod_YYYYMMDD_ai_chat.sql` - MySQL migration
6. `store/db/postgres/migration/prod_YYYYMMDD_ai_chat.sql` - PostgreSQL migration

**Implementation steps:**

1. Define Go structs in `store/` files:

   ```go
   type AIConversation struct {
       ID           int
       Name         string
       CreatorID    int
       LLMProvider  string
       LLMModel     string
       SystemPrompt string
       CreatedTs    int64
       UpdatedTs    int64
   }

   type AIMessage struct {
       ID             int
       ConversationID int
       Role           string // "user" or "assistant"
       Content        string
       Tokens         int
       CreatedTs      int64
   }

   type AIProviderConfig struct {
       ID          int
       Name        string
       DisplayName string
       APIKey      string
       APIEndpoint string
       Config      string // JSON
       Enabled     bool
       CreatedTs   int64
       UpdatedTs   int64
   }
   ```

2. Add methods to `store.Driver` interface in `store/driver.go`
3. Implement driver-specific methods in:

   - `store/db/sqlite/ai_conversation.go`
   - `store/db/mysql/ai_conversation.go`
   - `store/db/postgres/ai_conversation.go`
     (Repeat for messages and provider configs)

4. Create migration SQL files with CREATE TABLE statements
5. Update schema version in `store/migrator.go`

#### 1.2 Create Store Tests

Create `store/test/ai_conversation_test.go` with tests for:

- Creating conversations
- Listing user conversations
- Adding messages
- Retrieving conversation history
- Deleting conversations

---

### Phase 2: Protocol Buffer Definitions & API Services

#### 2.1 Define Proto Service

Create `proto/api/v1/ai_service.proto`:

```protobuf
syntax = "proto3";

package memos.api.v1;

import "api/v1/common.proto";
import "google/api/annotations.proto";
import "google/api/client.proto";
import "google/api/field_behavior.proto";
import "google/api/resource.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

option go_package = "gen/api/v1";

service AIService {
  // CreateConversation creates a new AI conversation
  rpc CreateConversation(CreateConversationRequest) returns (Conversation) {
    option (google.api.http) = {
      post: "/api/v1/ai/conversations"
      body: "*"
    };
  }

  // ListConversations lists all conversations for the current user
  rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse) {
    option (google.api.http) = {
      get: "/api/v1/ai/conversations"
    };
  }

  // GetConversation retrieves a specific conversation
  rpc GetConversation(GetConversationRequest) returns (Conversation) {
    option (google.api.http) = {
      get: "/api/v1/ai/conversations/{conversation_id}"
    };
  }

  // DeleteConversation deletes a conversation
  rpc DeleteConversation(DeleteConversationRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      delete: "/api/v1/ai/conversations/{conversation_id}"
    };
  }

  // UpdateConversation updates conversation metadata
  rpc UpdateConversation(UpdateConversationRequest) returns (Conversation) {
    option (google.api.http) = {
      patch: "/api/v1/ai/conversations/{conversation_id}"
      body: "*"
    };
  }

  // SendMessage sends a message and streams the AI response
  rpc SendMessage(SendMessageRequest) returns (stream MessageChunk) {
    option (google.api.http) = {
      post: "/api/v1/ai/conversations/{conversation_id}/messages"
      body: "*"
    };
  }

  // ListMessages lists messages in a conversation
  rpc ListMessages(ListMessagesRequest) returns (ListMessagesResponse) {
    option (google.api.http) = {
      get: "/api/v1/ai/conversations/{conversation_id}/messages"
    };
  }

  // ListProviders lists available LLM providers
  rpc ListProviders(ListProvidersRequest) returns (ListProvidersResponse) {
    option (google.api.http) = {
      get: "/api/v1/ai/providers"
    };
  }

  // ConfigureProvider configures an LLM provider (admin only)
  rpc ConfigureProvider(ConfigureProviderRequest) returns (Provider) {
    option (google.api.http) = {
      post: "/api/v1/ai/providers"
      body: "*"
    };
  }
}

message Conversation {
  int32 id = 1;
  string name = 2;
  int32 creator_id = 3;
  string llm_provider = 4;
  string llm_model = 5;
  string system_prompt = 6;
  google.protobuf.Timestamp created_time = 7;
  google.protobuf.Timestamp updated_time = 8;
  int32 message_count = 9; // Computed field
}

message Message {
  int32 id = 1;
  int32 conversation_id = 2;
  string role = 3; // "user" or "assistant"
  string content = 4;
  int32 tokens = 5;
  google.protobuf.Timestamp created_time = 6;
}

message Provider {
  string name = 1;
  string display_name = 2;
  string api_endpoint = 3;
  repeated string available_models = 4;
  bool enabled = 5;
  bool configured = 6; // Whether API key is set
}

message CreateConversationRequest {
  string name = 1;
  string llm_provider = 2;
  string llm_model = 3;
  string system_prompt = 4;
}

message ListConversationsRequest {
  int32 page_size = 1;
  string page_token = 2;
}

message ListConversationsResponse {
  repeated Conversation conversations = 1;
  string next_page_token = 2;
}

message GetConversationRequest {
  int32 conversation_id = 1;
}

message DeleteConversationRequest {
  int32 conversation_id = 1;
}

message UpdateConversationRequest {
  int32 conversation_id = 1;
  string name = 2;
  string system_prompt = 3;
}

message SendMessageRequest {
  int32 conversation_id = 1;
  string content = 2;
}

message MessageChunk {
  string content = 1; // Partial content for streaming
  bool is_final = 2; // True for the last chunk
  Message message = 3; // Full message when is_final=true
}

message ListMessagesRequest {
  int32 conversation_id = 1;
  int32 page_size = 2;
  string page_token = 3;
}

message ListMessagesResponse {
  repeated Message messages = 1;
  string next_page_token = 2;
}

message ListProvidersRequest {}

message ListProvidersResponse {
  repeated Provider providers = 1;
}

message ConfigureProviderRequest {
  string name = 1;
  string display_name = 2;
  string api_key = 3;
  string api_endpoint = 4;
  string config = 5; // JSON string
  bool enabled = 6;
}
```

**Run code generation:**

```bash
cd proto
buf generate
```

This generates:

- Go code: `proto/gen/api/v1/ai_service.pb.go` and `ai_service_grpc.pb.go`
- TypeScript: `web/src/types/proto/api/v1/ai_service.ts`

#### 2.2 Implement Backend Service

Create `server/router/api/v1/ai_service.go`:

**Key implementation points:**

1. Implement all RPC methods from the proto definition
2. Use dependency injection for LLM client abstraction
3. Handle authentication via existing middleware
4. Implement streaming for `SendMessage` using gRPC streaming
5. Validate user permissions (users can only access their own conversations)
6. Add ACL rules to `acl_config.go` if needed

**Example structure:**

```go
type AIServiceServer struct {
    pb.UnimplementedAIServiceServer
    Store      *store.Store
    LLMManager *ai.LLMManager // New package for LLM abstraction
}

func (s *AIServiceServer) SendMessage(req *pb.SendMessageRequest, stream pb.AIService_SendMessageServer) error {
    // 1. Authenticate user
    // 2. Validate conversation ownership
    // 3. Save user message to database
    // 4. Get LLM client for conversation's provider
    // 5. Stream response from LLM
    // 6. Save assistant message to database
    // 7. Send final chunk with complete message
}
```

#### 2.3 Create LLM Abstraction Layer

Create new package `internal/ai/`:

**Files:**

- `internal/ai/manager.go` - LLM provider manager
- `internal/ai/provider.go` - Provider interface
- `internal/ai/openai.go` - OpenAI provider implementation
- `internal/ai/deepseek.go` - Deepseek provider implementation
- `internal/ai/types.go` - Common types

**Provider interface:**

```go
type Provider interface {
    Name() string
    SendMessage(ctx context.Context, req ChatRequest) (ChatResponse, error)
    StreamMessage(ctx context.Context, req ChatRequest) (<-chan string, <-chan error)
    GetModels() []string
}

type ChatRequest struct {
    Messages     []Message
    Model        string
    SystemPrompt string
    Temperature  float32
}

type Message struct {
    Role    string
    Content string
}
```

**Implementation notes:**

- Use standard HTTP clients or official SDKs
- Handle rate limiting and retries
- Support streaming responses
- Add configuration for timeouts, max tokens, etc.

---

### Phase 3: Frontend Implementation

#### 3.1 Add Navigation Route

**Update `web/src/router/index.tsx`:**

1. Import new AI Chat page (lazy loaded)
2. Add route to `Routes` enum: `AI = "/ai"`
3. Add route configuration under `MainLayout` children

```typescript
const AIChat = lazy(() => import("@/pages/AIChat"));

// In Routes enum
export enum Routes {
  ROOT = "/",
  EXPLORE = "/explore",
  ATTACHMENTS = "/attachments",
  INBOX = "/inbox",
  ARCHIVED = "/archived",
  AI = "/ai", // Add this
  SETTING = "/setting",
  AUTH = "/auth",
}

// In router configuration
{
  path: Routes.AI,
  element: (
    <Suspense fallback={<Loading />}>
      <AIChat />
    </Suspense>
  ),
}
```

**Update `web/src/components/Navigation.tsx`:**

Add AI navigation link to the `navLinks` array:

```typescript
import { BotIcon } from "lucide-react"; // Add this import

const aiNavLink: NavLinkItem = {
  id: "header-ai",
  path: Routes.AI,
  title: t("common.ai"), // Add translation
  icon: <BotIcon className="w-6 h-auto shrink-0" />,
};

const navLinks: NavLinkItem[] = currentUser
  ? [homeNavLink, exploreNavLink, attachmentsNavLink, inboxNavLink, aiNavLink]
  : [exploreNavLink, signInNavLink];
```

**Add translations** in `web/src/locales/*/index.ts`:

```typescript
// en/index.ts
ai: "AI",
ai_chat: "AI Chat",
new_conversation: "New Conversation",
// ... more translations
```

#### 3.2 Create MobX Store

Create `web/src/store/aiStore.ts`:

```typescript
import { makeAutoObservable } from "mobx";
import { aiServiceClient } from "@/grpcweb";
import { Conversation, Message } from "@/types/proto/api/v1/ai_service";

class AIStore {
  conversations: Conversation[] = [];
  currentConversation: Conversation | null = null;
  messages: Message[] = [];
  isStreaming: boolean = false;
  streamingContent: string = "";

  constructor() {
    makeAutoObservable(this);
  }

  async fetchConversations() {
    const response = await aiServiceClient.listConversations({});
    this.conversations = response.conversations;
  }

  async createConversation(name: string, provider: string, model: string) {
    const conversation = await aiServiceClient.createConversation({
      name,
      llmProvider: provider,
      llmModel: model,
    });
    this.conversations.unshift(conversation);
    return conversation;
  }

  async loadConversation(conversationId: number) {
    const [conversation, messagesResponse] = await Promise.all([
      aiServiceClient.getConversation({ conversationId }),
      aiServiceClient.listMessages({ conversationId }),
    ]);
    this.currentConversation = conversation;
    this.messages = messagesResponse.messages;
  }

  async sendMessage(content: string) {
    if (!this.currentConversation) return;

    this.isStreaming = true;
    this.streamingContent = "";

    try {
      const stream = aiServiceClient.sendMessage({
        conversationId: this.currentConversation.id,
        content,
      });

      for await (const chunk of stream) {
        if (chunk.isFinal && chunk.message) {
          this.messages.push(chunk.message);
          this.streamingContent = "";
        } else {
          this.streamingContent += chunk.content;
        }
      }
    } finally {
      this.isStreaming = false;
    }
  }

  async deleteConversation(conversationId: number) {
    await aiServiceClient.deleteConversation({ conversationId });
    this.conversations = this.conversations.filter(
      (c) => c.id !== conversationId
    );
    if (this.currentConversation?.id === conversationId) {
      this.currentConversation = null;
      this.messages = [];
    }
  }
}

export const aiStore = new AIStore();
```

Register store in `web/src/store/index.ts`.

#### 3.3 Create AI Chat Page

Create `web/src/pages/AIChat.tsx`:

**Layout structure:**

- Left sidebar: Conversation list
- Main area: Message thread
- Bottom: Input box

**Key components:**

```typescript
const AIChat = observer(() => {
  const store = aiStore;

  useEffect(() => {
    store.fetchConversations();
  }, []);

  return (
    <div className="w-full h-screen flex">
      <ConversationSidebar />
      <ChatArea />
    </div>
  );
});
```

#### 3.4 Create UI Components

**Components to create:**

1. **`web/src/components/AI/ConversationSidebar.tsx`**

   - List of conversations
   - "New Chat" button
   - Delete conversation action
   - Search/filter conversations

2. **`web/src/components/AI/ChatArea.tsx`**

   - Message thread display
   - Streaming message indicator
   - Scroll to bottom behavior
   - Empty state

3. **`web/src/components/AI/MessageBubble.tsx`**

   - User vs Assistant styling
   - Markdown rendering (reuse existing MemoContent renderer)
   - Copy message button
   - Timestamp display

4. **`web/src/components/AI/ChatInput.tsx`**

   - Multi-line textarea
   - Send button (disabled during streaming)
   - Stop generation button
   - Character/token count
   - Model selector dropdown

5. **`web/src/components/AI/NewConversationDialog.tsx`**

   - Provider selection
   - Model selection
   - System prompt input (optional)
   - Conversation name input

6. **`web/src/components/AI/ProviderSettings.tsx`** (in Settings page)
   - Admin-only section
   - Configure API keys for providers
   - Enable/disable providers
   - Test connection button

**Design considerations:**

- Use Radix UI components (Dialog, DropdownMenu, etc.)
- Follow existing design patterns from MemoView
- Use Tailwind CSS utilities
- Support dark mode
- Mobile responsive (drawer for sidebar on small screens)

#### 3.5 Update Settings Page

Add AI settings section to `web/src/pages/Setting.tsx`:

- Link to provider configuration (admin only)
- User preferences for default model
- API key input for personal use (if allowed)

---

### Phase 4: Integration & Polish

#### 4.1 Add gRPC-Web Client Setup

Update `web/src/grpcweb.ts` to include AI service client:

```typescript
import { AIServiceClient } from "@/types/proto/api/v1/ai_service.client";

export const aiServiceClient = new AIServiceClient(
  transport,
  createClientOptions()
);
```

#### 4.2 Update MainLayout Context

Update `web/src/layouts/MainLayout.tsx` to handle AI route context:

```typescript
const context: MemoExplorerContext = useMemo(() => {
  if (location.pathname === Routes.ROOT) return "home";
  if (location.pathname === Routes.EXPLORE) return "explore";
  if (location.pathname === Routes.AI) return "ai"; // Add this
  // ... rest of logic
}, [location.pathname]);
```

Consider whether AI page needs MemoExplorer sidebar or custom sidebar.

#### 4.3 Implement Security & Validation

**Backend:**

- Validate conversation ownership in all operations
- Sanitize user inputs
- Rate limit API calls per user
- Encrypt API keys in database
- Add audit logging for AI interactions

**Frontend:**

- Validate message length before sending
- Handle network errors gracefully
- Show loading states
- Prevent double submissions

#### 4.4 Add Error Handling

**Backend:**

- Wrap LLM API errors with user-friendly messages
- Handle timeout scenarios
- Log errors for debugging
- Return appropriate gRPC status codes

**Frontend:**

- Toast notifications for errors
- Retry mechanisms for transient failures
- Offline detection
- Stream interruption handling

#### 4.5 Performance Optimization

**Backend:**

- Index conversation and message tables by creator_id and created_ts
- Implement pagination for message history
- Cache provider configurations
- Connection pooling for LLM APIs

**Frontend:**

- Virtual scrolling for long message lists
- Lazy load old messages
- Debounce input changes
- Optimize re-renders with React.memo

#### 4.6 Testing

**Backend tests:**

```bash
go test ./server/router/api/v1/ai_service_test.go
go test ./internal/ai/...
go test ./store/test/ai_*_test.go
```

**Frontend:**

- Manual testing across browsers
- Mobile responsiveness testing
- Dark mode verification
- Streaming behavior validation

---

### Phase 5: Documentation & Deployment

#### 5.1 Add API Documentation

Update `proto/api/v1/README.md` with AI service documentation:

- Endpoint descriptions
- Request/response examples
- Authentication requirements
- Rate limits

#### 5.2 Update User Documentation

Create user guide for AI chat feature:

- How to start a conversation
- Selecting providers and models
- Managing conversation history
- Best practices for prompts

#### 5.3 Admin Configuration Guide

Document for system administrators:

- How to configure LLM providers
- API key setup instructions
- Cost management considerations
- Privacy and data retention policies

#### 5.4 Migration Guide

Provide migration instructions:

1. Run database migrations
2. Configure at least one LLM provider
3. Restart server
4. Clear browser cache for frontend updates

#### 5.5 Environment Variables

Add new environment variables:

```bash
# AI Feature Configuration
MEMOS_AI_ENABLED=true                    # Enable/disable AI feature
MEMOS_AI_DEFAULT_PROVIDER=openai         # Default LLM provider
MEMOS_AI_OPENAI_API_KEY=sk-...          # OpenAI API key
MEMOS_AI_DEEPSEEK_API_KEY=sk-...        # Deepseek API key
MEMOS_AI_RATE_LIMIT=100                  # Messages per user per day
```

---

## Technical Considerations

### LLM Provider Support

**OpenAI Integration:**

- Use official `openai-go` SDK
- Support models: GPT-4, GPT-3.5-turbo, GPT-4-turbo
- Implement streaming via SSE

**Deepseek Integration:**

- Use OpenAI-compatible API
- Endpoint: `https://api.deepseek.com/v1`
- Support models: deepseek-chat, deepseek-coder

**Extensibility:**

- Design provider interface for easy addition of new providers
- Consider: Anthropic Claude, Google Gemini, local models (Ollama)

### Streaming Implementation

**Backend (gRPC streaming):**

```go
func (s *AIServiceServer) SendMessage(req *pb.SendMessageRequest, stream pb.AIService_SendMessageServer) error {
    // Stream chunks as they arrive from LLM
    for chunk := range llmStream {
        if err := stream.Send(&pb.MessageChunk{
            Content: chunk,
            IsFinal: false,
        }); err != nil {
            return err
        }
    }
    // Send final message
    return stream.Send(&pb.MessageChunk{
        IsFinal: true,
        Message: savedMessage,
    })
}
```

**Frontend (async iteration):**

```typescript
const stream = aiServiceClient.sendMessage(request);
for await (const chunk of stream) {
  // Update UI with streaming content
}
```

### Database Indexing

Add indexes for query performance:

```sql
CREATE INDEX idx_ai_conversation_creator_id ON ai_conversation(creator_id);
CREATE INDEX idx_ai_conversation_updated_ts ON ai_conversation(updated_ts DESC);
CREATE INDEX idx_ai_message_conversation_id ON ai_message(conversation_id);
CREATE INDEX idx_ai_message_created_ts ON ai_message(created_ts);
```

### Token Counting

Implement token counting for cost tracking:

- Use `tiktoken` library for OpenAI models
- Store token counts in message records
- Display usage statistics in UI
- Add per-user limits if needed

---

## UI/UX Guidelines

### Design Inspiration

Reference ChatGPT UI patterns:

- Clean, minimal interface
- Clear distinction between user and AI messages
- Smooth streaming animation
- Copy message button
- Regenerate response option

### Color Scheme

Follow existing Memos design:

- User messages: subtle background (bg-secondary)
- AI messages: transparent or slightly different shade
- Code blocks: use existing MemoContent rendering
- Streaming indicator: subtle pulse animation

### Accessibility

- Keyboard navigation support
- Screen reader friendly
- Focus management for modals
- ARIA labels for actions
- High contrast mode support

### Responsive Design

**Desktop (lg):**

- Sidebar: 280px fixed width
- Chat area: flex-grow
- Input: sticky bottom

**Tablet (md):**

- Collapsible sidebar or drawer
- Full-width chat area

**Mobile (sm):**

- Bottom sheet for conversation list
- Full-screen chat interface
- Optimized input for touch

---

## Security & Privacy

### Data Protection

- **Encryption at rest:** Encrypt API keys in database
- **Encryption in transit:** HTTPS/TLS for all API calls
- **Access control:** Users can only access their own conversations
- **Audit logging:** Log all AI interactions for compliance

### Privacy Considerations

- **Data retention:** Allow users to delete conversations
- **Export option:** Provide conversation export feature
- **Disclaimer:** Inform users about data sent to third-party LLMs
- **Opt-out:** Make AI feature optional per-user

### Rate Limiting

Implement rate limits to prevent abuse:

- Per-user message limits (e.g., 100/day)
- Per-conversation limits
- Global instance limits
- Graceful degradation when limits exceeded

---

## Testing Checklist

### Backend

- [ ] Database migrations run successfully on all drivers
- [ ] CRUD operations for conversations work correctly
- [ ] Message storage and retrieval tested
- [ ] Streaming responses work properly
- [ ] Provider configurations are encrypted
- [ ] Authentication and authorization enforced
- [ ] Rate limiting functions correctly
- [ ] Error handling covers edge cases

### Frontend

- [ ] Navigation to AI page works
- [ ] Conversation list displays correctly
- [ ] New conversation dialog functions
- [ ] Message sending and receiving works
- [ ] Streaming text displays smoothly
- [ ] Markdown rendering in messages
- [ ] Dark mode support verified
- [ ] Mobile responsive layout tested
- [ ] Error messages display appropriately
- [ ] Loading states are clear

### Integration

- [ ] End-to-end message flow works
- [ ] Multiple providers can be configured
- [ ] Switching between models works
- [ ] Conversation persistence verified
- [ ] Browser refresh maintains state
- [ ] Multiple concurrent conversations tested

---

## Future Enhancements

### Phase 2 Features (Post-MVP)

- [ ] **Context-aware AI:** Allow AI to reference user's memos in responses
- [ ] **Memo generation:** Create memos from AI conversations
- [ ] **Voice input:** Add speech-to-text for messages
- [ ] **Image support:** Multi-modal AI with image inputs
- [ ] **Shared conversations:** Allow conversation sharing
- [ ] **AI assistants:** Pre-configured AI personas for different tasks
- [ ] **Prompt templates:** Library of useful prompts
- [ ] **Search conversations:** Full-text search across all messages
- [ ] **Export formats:** Export to Markdown, PDF, etc.
- [ ] **Usage analytics:** Detailed token usage and cost tracking

---

## Resources & References

### Documentation

- [gRPC Streaming Guide](https://grpc.io/docs/languages/go/basics/#server-side-streaming-rpc)
- [Protocol Buffers Style Guide](https://protobuf.dev/programming-guides/style/)
- [OpenAI API Documentation](https://platform.openai.com/docs/api-reference)
- [Deepseek API Documentation](https://platform.deepseek.com/api-docs/)

### Dependencies to Add

```bash
# Backend
go get github.com/sashabaranov/go-openai
go get github.com/tiktoken-go/tokenizer

# Frontend (if needed)
pnpm add @types/marked
```

### Similar Implementations

- Study how other note-taking apps integrate AI (Notion AI, Obsidian with AI plugins)
- Review open-source chatbot UI implementations
- Examine streaming chat implementations in React

---

## Success Criteria

The feature is complete when:

1. ✅ Users can create and manage AI conversations
2. ✅ Multiple LLM providers are supported (OpenAI, Deepseek minimum)
3. ✅ Streaming responses work smoothly
4. ✅ Conversation history is persisted and retrievable
5. ✅ UI matches the quality and polish of existing Memos features
6. ✅ Mobile experience is fully functional
7. ✅ Admin configuration interface is complete
8. ✅ All tests pass
9. ✅ Documentation is comprehensive
10. ✅ Security requirements are met

---

## Getting Started

Begin implementation by following phases sequentially:

1. **Start with Phase 1:** Create database schema and store layer
2. **Test thoroughly:** Ensure data persistence works before moving on
3. **Move to Phase 2:** Define APIs and implement backend services
4. **Build incrementally:** Get basic chat working before adding advanced features
5. **Polish in Phase 4:** Refine UX and handle edge cases
6. **Document everything:** Keep documentation up-to-date as you build

Remember to follow existing code patterns, write tests, and commit frequently with meaningful commit messages following Conventional Commits format.

Good luck! 🚀
