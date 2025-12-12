import { makeAutoObservable, runInAction } from "mobx";
import { aiServiceClient } from "@/grpcweb";
import type { Conversation, Message, Provider } from "@/types/proto/api/v1/ai_service";

class AIStore {
  conversations: Conversation[] = [];
  currentConversation: Conversation | null = null;
  messages: Message[] = [];
  providers: Provider[] = [];
  isStreaming: boolean = false;
  streamingContent: string = "";
  isLoadingConversations: boolean = false;
  isLoadingMessages: boolean = false;
  isChatOpen: boolean = false;
  chatViewMode: "floating" | "sidebar" = "floating";

  constructor() {
    makeAutoObservable(this);
  }

  setChatOpen(open: boolean) {
    this.isChatOpen = open;
  }

  setChatViewMode(mode: "floating" | "sidebar") {
    this.chatViewMode = mode;
  }

  async fetchProviders() {
    try {
      console.log("Fetching AI providers...");
      const response = await aiServiceClient.listProviders({});
      console.log("Providers response:", response);
      runInAction(() => {
        this.providers = response.providers;
        console.log("Providers set:", this.providers);
      });
    } catch (error) {
      console.error("Failed to fetch providers:", error);
      // Log more details about the error
      if (error && typeof error === 'object') {
        console.error("Error details:", {
          message: (error as any).message,
          code: (error as any).code,
          details: (error as any).details,
        });
      }
    }
  }

  async fetchConversations() {
    this.isLoadingConversations = true;
    try {
      const response = await aiServiceClient.listConversations({});
      runInAction(() => {
        this.conversations = response.conversations;
        this.isLoadingConversations = false;
      });
    } catch (error) {
      console.error("Failed to fetch conversations:", error);
      runInAction(() => {
        this.isLoadingConversations = false;
      });
    }
  }

  async createConversation(name: string, provider: string, model: string, systemPrompt?: string) {
    try {
      const conversation = await aiServiceClient.createConversation({
        name,
        llmProvider: provider,
        llmModel: model,
        systemPrompt: systemPrompt || "",
      });
      runInAction(() => {
        this.conversations.unshift(conversation);
      });
      return conversation;
    } catch (error) {
      console.error("Failed to create conversation:", error);
      throw error;
    }
  }

  async loadConversation(conversationId: number) {
    this.isLoadingMessages = true;
    try {
      const [conversation, messagesResponse] = await Promise.all([
        aiServiceClient.getConversation({ conversationId }),
        aiServiceClient.listMessages({ conversationId, pageSize: 0, pageToken: "" }),
      ]);
      runInAction(() => {
        this.currentConversation = conversation;
        this.messages = messagesResponse.messages;
        this.isLoadingMessages = false;
      });
    } catch (error) {
      console.error("Failed to load conversation:", error);
      runInAction(() => {
        this.isLoadingMessages = false;
      });
      throw error;
    }
  }

  async sendMessage(content: string) {
    if (!this.currentConversation) return;

    // Add user message to UI immediately
    const userMessage: Message = {
      id: 0,
      conversationId: this.currentConversation.id,
      role: "user",
      content,
      tokens: 0,
      createdTime: undefined,
    };
    runInAction(() => {
      this.messages.push(userMessage);
      this.isStreaming = true;
      this.streamingContent = "";
    });

    try {
      const stream = aiServiceClient.sendMessage({
        conversationId: this.currentConversation.id,
        content,
      });

      for await (const chunk of stream) {
        if (chunk.isFinal && chunk.message) {
          const finalMessage = chunk.message;
          runInAction(() => {
            // Replace user message with saved one (with ID)
            const userMsgIndex = this.messages.findIndex((m) => m.id === 0 && m.role === "user");
            if (userMsgIndex >= 0) {
              this.messages[userMsgIndex] = {
                ...this.messages[userMsgIndex],
                id: finalMessage.id - 1, // Approximate, backend should return user message too
              };
            }
            this.messages.push(finalMessage);
            this.streamingContent = "";
            this.isStreaming = false;
          });
        } else {
          runInAction(() => {
            this.streamingContent += chunk.content;
          });
        }
      }
    } catch (error) {
      console.error("Failed to send message:", error);
      runInAction(() => {
        this.isStreaming = false;
        this.streamingContent = "";
      });
      throw error;
    }
  }

  async deleteConversation(conversationId: number) {
    try {
      await aiServiceClient.deleteConversation({ conversationId });
      runInAction(() => {
        this.conversations = this.conversations.filter((c) => c.id !== conversationId);
        if (this.currentConversation?.id === conversationId) {
          this.currentConversation = null;
          this.messages = [];
        }
      });
    } catch (error) {
      console.error("Failed to delete conversation:", error);
      throw error;
    }
  }

  async updateConversation(conversationId: number, name: string, systemPrompt?: string) {
    try {
      const updated = await aiServiceClient.updateConversation({
        conversationId,
        name,
        systemPrompt: systemPrompt || "",
      });
      runInAction(() => {
        const index = this.conversations.findIndex((c) => c.id === conversationId);
        if (index >= 0) {
          this.conversations[index] = updated;
        }
        if (this.currentConversation?.id === conversationId) {
          this.currentConversation = updated;
        }
      });
    } catch (error) {
      console.error("Failed to update conversation:", error);
      throw error;
    }
  }

  clearCurrentConversation() {
    this.currentConversation = null;
    this.messages = [];
    this.streamingContent = "";
  }

  getAvailableProviders() {
    return this.providers.filter((p) => p.enabled && p.configured);
  }
}

export const aiStore = new AIStore();
