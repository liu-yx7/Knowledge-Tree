import { SendIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { aiStore } from "@/store";
import { useTranslate } from "@/utils/i18n";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface Props {
  onConversationCreated?: (id: number) => void;
}

const ChatInterface = observer(({ onConversationCreated }: Props) => {
  const t = useTranslate();
  const [inputValue, setInputValue] = useState("");
  const [selectedProvider, setSelectedProvider] = useState("");
  const [selectedModel, setSelectedModel] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const availableProviders = aiStore.getAvailableProviders();

  // Initialize provider selection
  useEffect(() => {
    if (aiStore.currentConversation) {
      // Use conversation's provider and model
      setSelectedProvider(aiStore.currentConversation.llmProvider);
      setSelectedModel(aiStore.currentConversation.llmModel);
    } else if (availableProviders.length > 0 && !selectedProvider) {
      // Default to first available provider
      const firstProvider = availableProviders[0];
      setSelectedProvider(firstProvider.name);
      if (firstProvider.availableModels.length > 0) {
        setSelectedModel(firstProvider.availableModels[0]);
      }
    }
  }, [aiStore.currentConversation, availableProviders]);

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [aiStore.messages, aiStore.streamingContent]);

  const handleSend = async () => {
    if (!inputValue.trim() || aiStore.isStreaming) return;

    const message = inputValue.trim();
    setInputValue("");

    try {
      // If no conversation exists, create one with the first message
      if (!aiStore.currentConversation) {
        if (!selectedProvider || !selectedModel) {
          console.error("No provider or model selected");
          return;
        }

        // Create conversation with first 50 chars of message as name
        const conversationName = message.length > 50 
          ? message.substring(0, 47) + "..." 
          : message;

        const conversation = await aiStore.createConversation(
          conversationName,
          selectedProvider,
          selectedModel
        );

        // Set as current conversation
        aiStore.currentConversation = conversation;
        
        // Notify parent to update URL
        onConversationCreated?.(conversation.id);
      }

      await aiStore.sendMessage(message);
    } catch (error) {
      console.error("Failed to send message:", error);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleProviderChange = (value: string) => {
    setSelectedProvider(value);
    const provider = availableProviders.find((p) => p.name === value);
    if (provider && provider.availableModels.length > 0) {
      setSelectedModel(provider.availableModels[0]);
    }
  };

  const selectedProviderData = availableProviders.find((p) => p.name === selectedProvider);
  const isExistingConversation = !!aiStore.currentConversation;

  return (
    <div className="flex-1 flex flex-col h-full">
      {/* Header */}
      {aiStore.currentConversation && (
        <div className="px-6 py-4 border-b">
          <h2 className="text-lg font-semibold">{aiStore.currentConversation.name}</h2>
          <p className="text-sm text-muted-foreground">
            {aiStore.currentConversation.llmProvider} · {aiStore.currentConversation.llmModel}
          </p>
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 px-6 py-4 overflow-y-auto" ref={scrollRef}>
        {aiStore.isLoadingMessages ? (
          <div className="flex items-center justify-center h-full">
            <p className="text-muted-foreground">{t("common.loading")}</p>
          </div>
        ) : aiStore.messages.length === 0 && !aiStore.currentConversation ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-center space-y-2">
              <h2 className="text-2xl font-bold">{t("ai.welcome")}</h2>
              <p className="text-muted-foreground">{t("ai.start-chatting")}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-6 max-w-4xl mx-auto">
            {aiStore.messages.map((message, index) => (
              <div
                key={`${message.id}-${index}`}
                className={cn(
                  "flex gap-4",
                  message.role === "user" ? "justify-end" : "justify-start"
                )}
              >
                <div
                  className={cn(
                    "max-w-[80%] rounded-2xl px-4 py-3",
                    message.role === "user"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted"
                  )}
                >
                  {message.role === "assistant" ? (
                    <div className="prose prose-sm dark:prose-invert max-w-none">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>
                        {message.content}
                      </ReactMarkdown>
                    </div>
                  ) : (
                    <p className="whitespace-pre-wrap">{message.content}</p>
                  )}
                </div>
              </div>
            ))}

            {/* Streaming message */}
            {aiStore.isStreaming && aiStore.streamingContent && (
              <div className="flex gap-4 justify-start">
                <div className="max-w-[80%] rounded-2xl px-4 py-3 bg-muted">
                  <div className="prose prose-sm dark:prose-invert max-w-none">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {aiStore.streamingContent}
                    </ReactMarkdown>
                  </div>
                </div>
              </div>
            )}

            {/* Typing indicator */}
            {aiStore.isStreaming && !aiStore.streamingContent && (
              <div className="flex gap-4 justify-start">
                <div className="rounded-2xl px-4 py-3 bg-muted">
                  <div className="flex gap-1">
                    <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: "0ms" }} />
                    <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: "150ms" }} />
                    <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: "300ms" }} />
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Input */}
      <div className="px-6 py-4 border-t bg-background">
        <div className="max-w-4xl mx-auto space-y-3">
          {/* Provider selection - always visible, disabled once conversation exists */}
          <div className="flex items-center gap-3 text-sm">
            <span className="text-muted-foreground">{t("ai.model")}:</span>
            <Select
              value={selectedProvider}
              onValueChange={handleProviderChange}
              disabled={isExistingConversation || aiStore.isStreaming}
            >
              <SelectTrigger className="h-8 w-[160px] text-xs">
                <SelectValue placeholder={t("ai.select-provider")} />
              </SelectTrigger>
              <SelectContent>
                {availableProviders.map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    {p.displayName}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={selectedModel}
              onValueChange={setSelectedModel}
              disabled={isExistingConversation || !selectedProvider || aiStore.isStreaming}
            >
              <SelectTrigger className="h-8 w-[180px] text-xs">
                <SelectValue placeholder={t("ai.select-model")} />
              </SelectTrigger>
              <SelectContent>
                {selectedProviderData?.availableModels.map((m) => (
                  <SelectItem key={m} value={m}>
                    {m}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {isExistingConversation && (
              <span className="text-xs text-muted-foreground italic">
                (locked for this conversation)
              </span>
            )}
          </div>

          {/* Input area */}
          <div className="flex gap-2">
            <Textarea
              ref={textareaRef}
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t("ai.type-message")}
              className="resize-none"
              rows={3}
              disabled={aiStore.isStreaming}
            />
            <Button
              onClick={handleSend}
              disabled={!inputValue.trim() || aiStore.isStreaming || (!isExistingConversation && (!selectedProvider || !selectedModel))}
              size="icon"
              className="h-auto"
            >
              <SendIcon className="w-5 h-5" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
});

export default ChatInterface;
