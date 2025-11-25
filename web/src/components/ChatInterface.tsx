import { SendIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { aiStore } from "@/store";
import { useTranslate } from "@/utils/i18n";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

const ChatInterface = observer(() => {
  const t = useTranslate();
  const [inputValue, setInputValue] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

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

  return (
    <div className="flex-1 flex flex-col h-full">
      {/* Header */}
      <div className="px-6 py-4 border-b">
        <h2 className="text-lg font-semibold">{aiStore.currentConversation?.name}</h2>
        <p className="text-sm text-muted-foreground">
          {aiStore.currentConversation?.llmProvider} · {aiStore.currentConversation?.llmModel}
        </p>
      </div>

      {/* Messages */}
      <div className="flex-1 px-6 py-4 overflow-y-auto" ref={scrollRef}>
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
      <div className="px-6 py-4 border-t">
        <div className="max-w-4xl mx-auto flex gap-2">
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
            disabled={!inputValue.trim() || aiStore.isStreaming}
            size="icon"
            className="h-auto"
          >
            <SendIcon className="w-5 h-5" />
          </Button>
        </div>
      </div>
    </div>
  );
});

export default ChatInterface;
