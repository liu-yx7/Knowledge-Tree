import { CopyIcon, SendIcon, PaperclipIcon, DatabaseIcon, BotIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";
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
import { CodeBlock } from "@/components/MemoContent/CodeBlock";

interface Props {
  onConversationCreated?: (id: number) => void;
  className?: string;
  hideHeader?: boolean;
}

const ChatInterface = observer(({ onConversationCreated, className, hideHeader }: Props) => {
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
        if (firstProvider.availableModels.includes("deepseek-chat")) {
          setSelectedModel("deepseek-chat");
        } else {
          setSelectedModel(firstProvider.availableModels[0]);
        }
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
          toast.error(t("ai.select-provider-model-first"));
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
    } catch (error: any) {
      console.error("Failed to send message:", error);
      toast.error(error.details || error.message || t("common.error"));
    }
  };

  const handleCopyMessage = (content: string) => {
    navigator.clipboard.writeText(content);
    toast.success(t("message.copied"));
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
      if (provider.availableModels.includes("deepseek-chat")) {
        setSelectedModel("deepseek-chat");
      } else {
        setSelectedModel(provider.availableModels[0]);
      }
    }
  };

  const isExistingConversation = !!aiStore.currentConversation;
  const isStartingPage = !aiStore.currentConversation && aiStore.messages.length === 0;

  const renderInputArea = (centered: boolean = false) => (
    <div className={cn(
      "flex flex-col border rounded-2xl bg-background shadow-sm transition-colors focus-within:border-primary",
      aiStore.isStreaming && "opacity-50 cursor-not-allowed"
    )}>
      <Textarea
        ref={textareaRef}
        value={inputValue}
        onChange={(e) => setInputValue(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={t("ai.type-message")}
        className="min-h-[60px] w-full resize-none border-0 bg-transparent shadow-none focus-visible:ring-0 p-4"
        rows={centered ? 4 : 3}
        disabled={aiStore.isStreaming}
      />
      
      <div className="flex items-center justify-between p-2 pl-3">
        <div className="flex items-center gap-1">
          <Select
            value={selectedProvider}
            onValueChange={handleProviderChange}
            disabled={isExistingConversation || aiStore.isStreaming}
          >
            <SelectTrigger className="h-8 border-0 shadow-none hover:bg-accent/50 w-auto gap-2 px-2 text-muted-foreground data-[state=open]:bg-accent/50">
              <BotIcon className="w-4 h-4" />
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

          <div className="w-px h-4 bg-border mx-1" />

          <Select value="all" disabled={aiStore.isStreaming}>
            <SelectTrigger className="h-8 border-0 shadow-none hover:bg-accent/50 w-auto gap-2 px-2 text-muted-foreground">
              <DatabaseIcon className="w-4 h-4" />
              <span className="text-xs">Source</span>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Memos</SelectItem>
            </SelectContent>
          </Select>

          <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground" disabled={aiStore.isStreaming}>
            <PaperclipIcon className="w-4 h-4" />
          </Button>
        </div>

        <Button
          onClick={handleSend}
          disabled={!inputValue.trim() || aiStore.isStreaming || (!isExistingConversation && (!selectedProvider || !selectedModel))}
          size="icon"
          className="h-8 w-8 rounded-lg"
        >
          <SendIcon className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  if (isStartingPage && !aiStore.isLoadingMessages) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center h-full p-4">
        <div className="w-full max-w-3xl flex flex-col gap-8 mb-20">
          <div className="flex flex-col items-center gap-4">
            <div className="p-4 rounded-3xl bg-muted/50">
              <BotIcon className="w-12 h-12 text-primary" />
            </div>
            <h2 className="text-2xl font-semibold">{t("ai.title")}</h2>
          </div>
          {renderInputArea(true)}
        </div>
      </div>
    );
  }

  return (
    <div className={cn("flex-1 flex flex-col h-full min-h-0", className)}>
      {/* Header */}
      {aiStore.currentConversation && !hideHeader && (
        <div className="px-6 py-4 border-b flex items-center justify-between bg-background z-10 shrink-0">
          <h2 className="text-lg font-semibold truncate">{aiStore.currentConversation.name}</h2>
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 px-6 py-4 overflow-y-auto min-h-0" ref={scrollRef}>
        {aiStore.isLoadingMessages ? (
          <div className="flex items-center justify-center h-full">
            <p className="text-muted-foreground">{t("common.loading")}</p>
          </div>
        ) : (
          <div className="space-y-6 max-w-4xl mx-auto">
            {aiStore.messages.map((message, index) => (
              <div
                key={`${message.id}-${index}`}
                className={cn(
                  "flex gap-4 group",
                  message.role === "user" ? "justify-end" : "justify-start"
                )}
              >
                <div
                  className={cn(
                    "max-w-[80%] rounded-2xl px-4 py-3 relative",
                    message.role === "user"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted"
                  )}
                >
                  {message.role === "assistant" ? (
                    <div className="prose prose-sm dark:prose-invert max-w-none">
                      <ReactMarkdown 
                        remarkPlugins={[remarkGfm]}
                        components={{
                          pre: CodeBlock,
                          a: ({ href, children, ...props }) => (
                            <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
                              {children}
                            </a>
                          ),
                        }}
                      >
                        {message.content}
                      </ReactMarkdown>
                    </div>
                  ) : (
                    <p className="whitespace-pre-wrap">{message.content}</p>
                  )}
                  
                  {message.role === "assistant" && (
                    <div className="absolute -bottom-6 left-0 opacity-0 group-hover:opacity-100 transition-opacity flex gap-2">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => handleCopyMessage(message.content)}
                      >
                        <CopyIcon className="w-3 h-3" />
                      </Button>
                    </div>
                  )}
                </div>
              </div>
            ))}

            {/* Streaming message */}
            {aiStore.isStreaming && aiStore.streamingContent && (
              <div className="flex gap-4 justify-start">
                <div className="max-w-[80%] rounded-2xl px-4 py-3 bg-muted">
                  <div className="prose prose-sm dark:prose-invert max-w-none">
                    <ReactMarkdown 
                      remarkPlugins={[remarkGfm]}
                      components={{
                        pre: CodeBlock,
                        a: ({ href, children, ...props }) => (
                          <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
                            {children}
                          </a>
                        ),
                      }}
                    >
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
      <div className="px-6 py-4 border-t bg-background shrink-0">
        <div className="max-w-4xl mx-auto">
          {renderInputArea(false)}
        </div>
      </div>
    </div>
  );
});

export default ChatInterface;