import { useState, useEffect, useRef } from "react";
import { create } from "@bufbuild/protobuf";
import { AIChatMessages, AIChatInput, AIChatEmptyState, AIChatConversationList } from "@/components/AIChat";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useConversations, useCreateConversation, useDeleteConversation, useMessages } from "@/hooks/useAIQueries";
import { useAIChatStream } from "@/hooks/useAIChatStream";
import { CreateConversationRequestSchema } from "@/types/proto/api/v1/ai_service_pb";

const AIChat = () => {
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const { data: conversations = [], isLoading: conversationsLoading } = useConversations();
  const { data: messages = [], isLoading: messagesLoading } = useMessages(activeConversationId || "");
  const createConversation = useCreateConversation();
  const deleteConversation = useDeleteConversation();
  const stream = useAIChatStream();

  // 自动滚动到底部
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, stream.streamingContent, stream.isStreaming]);

  const handleNewChat = async () => {
    try {
      stream.reset();
      const newConversation = await createConversation.mutateAsync(create(CreateConversationRequestSchema, {}));
      setActiveConversationId(newConversation.id);
    } catch (error) {
      console.error("Failed to create conversation:", error);
    }
  };

  const handleDeleteChat = async (id: string) => {
    try {
      await deleteConversation.mutateAsync(id);
      if (activeConversationId === id) {
        stream.reset();
        setActiveConversationId(null);
      }
    } catch (error) {
      console.error("Failed to delete conversation:", error);
    }
  };

  const handleSelectChat = (id: string) => {
    stream.reset();
    setActiveConversationId(id);
  };

  const handleSendMessage = async (content: string) => {
    if (!content.trim()) return;

    // 自动创建对话
    let conversationId = activeConversationId;
    if (!conversationId) {
      try {
        const newConversation = await createConversation.mutateAsync(create(CreateConversationRequestSchema, {}));
        conversationId = newConversation.id;
        setActiveConversationId(conversationId);
      } catch (error) {
        console.error("Failed to create conversation:", error);
        return;
      }
    }

    stream.sendStreamMessage(conversationId, content);
  };

  return (
    <div className="flex h-[calc(100vh-4rem)] sm:h-screen w-full">
      {/* 左侧对话列表 */}
      <div className="hidden md:block w-64 border-r shrink-0">
        <AIChatConversationList
          conversations={conversations}
          activeId={activeConversationId || undefined}
          onSelect={handleSelectChat}
          onDelete={handleDeleteChat}
          onNew={handleNewChat}
          isLoading={conversationsLoading}
          isCreating={createConversation.isPending}
        />
      </div>

      {/* 右侧聊天区域 */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {!activeConversationId ? (
          <AIChatEmptyState onNewChat={handleNewChat} isLoading={createConversation.isPending} />
        ) : (
          <>
            {/* 消息列表 */}
            <ScrollArea className="flex-1 overflow-hidden" ref={scrollRef}>
              <AIChatMessages
                messages={messages}
                isLoading={messagesLoading}
                isStreaming={stream.isStreaming}
                streamingContent={stream.streamingContent}
                streamingReasoning={stream.reasoningContent}
                streamingReferences={stream.references}
              />
            </ScrollArea>

            {/* 错误提示 */}
            {stream.error && (
              <div className="px-4 py-2 text-sm text-destructive bg-destructive/10 border-t">
                {stream.error}
              </div>
            )}

            {/* 输入区域 */}
            <AIChatInput
              onSend={handleSendMessage}
              onAbort={stream.abort}
              disabled={createConversation.isPending}
              isStreaming={stream.isStreaming}
            />
          </>
        )}
      </div>
    </div>
  );
};

export default AIChat;
