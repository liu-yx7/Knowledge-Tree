import { useEffect, useRef } from "react";
import { create } from "@bufbuild/protobuf";
import { ExternalLink, MessageSquarePlus, Minus } from "lucide-react";
import { Link } from "react-router-dom";
import { AIChatMessages, AIChatInput, AIChatEmptyState } from "@/components/AIChat";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAISidebar } from "@/contexts/AISidebarContext";
import { useConversation, useConversations, useCreateConversation, useMessages } from "@/hooks/useAIQueries";
import { useAIChatStream } from "@/hooks/useAIChatStream";
import { CreateConversationRequestSchema } from "@/types/proto/api/v1/ai_service_pb";

const AIChatSidebarContent = () => {
  const { activeConversationId, setActiveConversation, closeSidebar } = useAISidebar();
  const viewportRef = useRef<HTMLDivElement>(null);

  const { data: conversations = [] } = useConversations();
  const { isLoading: conversationLoading } = useConversation(activeConversationId || "");
  const { data: messages = [], isLoading: messagesLoading } = useMessages(activeConversationId || "");
  const createConversation = useCreateConversation();
  const stream = useAIChatStream();

  // 自动滚动到底部 — 必须操作 Viewport（Radix ScrollArea 实际可滚动元素）
  useEffect(() => {
    const viewport = viewportRef.current;
    if (viewport) {
      viewport.scrollTop = viewport.scrollHeight;
    }
  }, [messages, stream.streamingContent, stream.isStreaming]);

  const handleNewChat = async () => {
    try {
      stream.reset();
      const newConversation = await createConversation.mutateAsync(create(CreateConversationRequestSchema, {}));
      setActiveConversation(newConversation.id);
    } catch (error) {
      console.error("Failed to create conversation:", error);
    }
  };

  const handleSendMessage = async (content: string) => {
    if (!content.trim()) return;

    // 自动创建对话
    let conversationId = activeConversationId;
    if (!conversationId) {
      try {
        const newConversation = await createConversation.mutateAsync(create(CreateConversationRequestSchema, {}));
        conversationId = newConversation.id;
        setActiveConversation(conversationId);
      } catch (error) {
        console.error("Failed to create conversation:", error);
        return;
      }
    }

    // 使用 SSE 流式发送
    stream.sendStreamMessage(conversationId, content);
  };

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between p-3 border-b shrink-0">
        <div className="flex items-center gap-2">
          <h3 className="font-semibold text-sm">AI Assistant</h3>
          <Link to="/ai" onClick={closeSidebar}>
            <Button variant="ghost" size="icon" className="h-6 w-6" title="Open full AI page">
              <ExternalLink className="h-3 w-3" />
            </Button>
          </Link>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={handleNewChat}
            disabled={createConversation.isPending}
            title="New chat"
          >
            <MessageSquarePlus className="h-4 w-4" />
          </Button>
          {conversations.length > 0 && (
            <Select value={activeConversationId || ""} onValueChange={setActiveConversation}>
              <SelectTrigger className="h-7 w-28 text-xs">
                <SelectValue placeholder="Select chat" />
              </SelectTrigger>
              <SelectContent>
                {conversations.slice(0, 10).map((conv) => (
                  <SelectItem key={conv.id} value={conv.id} className="text-xs">
                    {conv.title || "New Chat"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={closeSidebar} title="Minimize">
            <Minus className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Messages */}
      <ScrollArea className="flex-1" viewportRef={viewportRef}>
        {!activeConversationId ? (
          <AIChatEmptyState onNewChat={handleNewChat} compact isLoading={createConversation.isPending} />
        ) : (
          <AIChatMessages
            messages={messages}
            isLoading={conversationLoading || messagesLoading}
            isStreaming={stream.isStreaming}
            streamingContent={stream.streamingContent}
            streamingReasoning={stream.reasoningContent}
            streamingReferences={stream.references}
            optimisticUserMessage={stream.optimisticUserMessage}
            compact
          />
        )}
      </ScrollArea>

      {/* Error */}
      {stream.error && (
        <div className="px-3 py-2 text-xs text-destructive bg-destructive/10 border-t">
          {stream.error}
        </div>
      )}

      {/* Input */}
      <div className="shrink-0 border-t">
        <AIChatInput
          onSend={handleSendMessage}
          onAbort={stream.abort}
          disabled={createConversation.isPending}
          isStreaming={stream.isStreaming}
          compact
        />
      </div>
    </div>
  );
};

export default AIChatSidebarContent;
