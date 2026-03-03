import { useState, useRef } from "react";
import { Bot, ChevronDown, ChevronRight } from "lucide-react";
import type { Message } from "@/types/proto/api/v1/ai_service_pb";
import { MessageRole } from "@/types/proto/api/v1/ai_service_pb";
import { cn } from "@/lib/utils";
import MarkdownRenderer from "@/components/MarkdownRenderer";
import ReferenceList, { type ReferenceItem } from "./ReferenceList";
import type { StreamReference } from "@/hooks/useAIChatStream";

// ==================== 类型定义 ====================

interface AIChatMessagesProps {
  messages: Message[];
  isLoading: boolean;
  /** 正在流式传输 */
  isStreaming?: boolean;
  /** 流式传输中的增量文本 */
  streamingContent?: string;
  /** 流式传输中的思考链文本 */
  streamingReasoning?: string;
  /** 流式传输完成后的引用列表 */
  streamingReferences?: StreamReference[];
  /** 乐观用户消息（发送后立即展示，防止空白闪烁） */
  optimisticUserMessage?: string | null;
  compact?: boolean;
}

// ==================== 辅助函数 ====================

/** 将后端 ParsedReference JSON 转换为 ReferenceItem */
function parseReferencesFromJSON(json: string): ReferenceItem[] {
  if (!json) return [];
  try {
    const refs: StreamReference[] = JSON.parse(json);
    return refs.map(toReferenceItem);
  } catch {
    return [];
  }
}

/** StreamReference → ReferenceItem 转换 */
function toReferenceItem(ref: StreamReference): ReferenceItem {
  return {
    memoUid: ref.memo_uid,
    attachmentUid: ref.attachment_uid,
    type: ref.type,
    title: ref.title,
    contentSnippet: ref.content_snippet,
    similarity: ref.similarity,
    imageId: ref.image_id,
    docType: ref.doc_type,
    documentName: ref.document_name,
  };
}

// ==================== 思考链折叠组件 ====================

function ReasoningBlock({ content, compact }: { content: string; compact: boolean }) {
  const [isOpen, setIsOpen] = useState(false);

  if (!content) return null;

  return (
    <div className={cn("mb-2 border border-dashed rounded-md", compact ? "p-1.5" : "p-2", "border-amber-300/50 dark:border-amber-600/30 bg-amber-50/50 dark:bg-amber-900/10")}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={cn("flex items-center gap-1 text-amber-700 dark:text-amber-400 w-full text-left", compact ? "text-[10px]" : "text-xs")}
      >
        {isOpen ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
        <span className="font-medium">思考过程</span>
      </button>
      {isOpen && (
        <div className={cn("mt-1 text-amber-800/80 dark:text-amber-300/70 whitespace-pre-wrap", compact ? "text-[10px]" : "text-xs")}>
          {content}
        </div>
      )}
    </div>
  );
}

// ==================== 主组件 ====================

const AIChatMessages = ({
  messages,
  isLoading,
  isStreaming = false,
  streamingContent = "",
  streamingReasoning = "",
  streamingReferences = [],
  optimisticUserMessage = null,
  compact = false,
}: AIChatMessagesProps) => {
  // 安全网：一旦对话中出现过消息，永远不再回退至空状态（防止 React Query refetch 间隙闪烁）
  const hasEverHadMessagesRef = useRef(false);
  if (messages.length > 0 || optimisticUserMessage) {
    hasEverHadMessagesRef.current = true;
  }

  if (isLoading) {
    return <div className="text-center text-muted-foreground py-8">Loading messages...</div>;
  }

  if (messages.length === 0 && !isStreaming && !optimisticUserMessage && !hasEverHadMessagesRef.current) {
    return <div className="text-center text-muted-foreground py-8">Start the conversation by typing a message below.</div>;
  }

  return (
    <div className={cn("space-y-4", compact ? "p-3" : "max-w-3xl mx-auto p-4")}>
      {/* 历史消息 */}
      {messages.map((message) => {
        const isUser = message.role === MessageRole.USER;
        const refs = parseReferencesFromJSON(message.referencesJson);

        return (
          <div key={message.id} className={cn("flex gap-3 rounded-lg", compact ? "p-2" : "p-4", isUser ? "bg-primary/5" : "bg-muted/50")}>
            <div
              className={cn(
                "rounded-full flex items-center justify-center shrink-0",
                compact ? "w-6 h-6" : "w-8 h-8",
                isUser ? "bg-primary text-primary-foreground" : "bg-muted-foreground/20",
              )}
            >
              {isUser ? <span className={compact ? "text-xs" : "text-sm"}>U</span> : <Bot className={compact ? "w-3 h-3" : "w-4 h-4"} />}
            </div>
            <div className="flex-1 min-w-0 overflow-x-hidden">
              {/* 思考链（assistant 消息可选） */}
              {!isUser && message.reasoningContent && <ReasoningBlock content={message.reasoningContent} compact={compact} />}
              {/* 消息内容 */}
              <div className={cn("prose dark:prose-invert max-w-none break-words", compact ? "prose-xs" : "prose-sm")}>
                <MarkdownRenderer content={message.content} references={isUser ? undefined : refs} />
              </div>
              {/* 引用来源 */}
              {!isUser && refs.length > 0 && <ReferenceList references={refs} compact={compact} />}
            </div>
          </div>
        );
      })}

      {/* 乐观用户消息（React Query 尚未返回时立即展示） */}
      {optimisticUserMessage && (
        <div className={cn("flex gap-3 rounded-lg bg-primary/5", compact ? "p-2" : "p-4")}>
          <div
            className={cn(
              "rounded-full flex items-center justify-center shrink-0 bg-primary text-primary-foreground",
              compact ? "w-6 h-6" : "w-8 h-8",
            )}
          >
            <span className={compact ? "text-xs" : "text-sm"}>U</span>
          </div>
          <div className="flex-1 min-w-0 overflow-x-hidden">
            <div className={cn("prose dark:prose-invert max-w-none break-words", compact ? "prose-xs" : "prose-sm")}>
              <MarkdownRenderer content={optimisticUserMessage} />
            </div>
          </div>
        </div>
      )}

      {/* 流式传输中的 assistant 消息 */}
      {isStreaming && (
        <div className={cn("flex gap-3 rounded-lg bg-muted/50", compact ? "p-2" : "p-4")}>
          <div className={cn("rounded-full flex items-center justify-center shrink-0 bg-muted-foreground/20", compact ? "w-6 h-6" : "w-8 h-8")}>
            <Bot className={compact ? "w-3 h-3" : "w-4 h-4"} />
          </div>
          <div className="flex-1 min-w-0 overflow-x-hidden">
            {/* 流式思考链 */}
            {streamingReasoning && <ReasoningBlock content={streamingReasoning} compact={compact} />}
            {/* 流式内容 */}
            {streamingContent ? (
              <div className={cn("prose dark:prose-invert max-w-none break-words", compact ? "prose-xs" : "prose-sm")}>
                <MarkdownRenderer content={streamingContent} references={streamingReferences.length > 0 ? streamingReferences.map(toReferenceItem) : undefined} />
                {/* 打字机光标 */}
                <span className="inline-block w-0.5 h-4 bg-foreground/70 animate-pulse ml-0.5 align-text-bottom" />
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <div className="flex gap-1">
                  <span className="w-1.5 h-1.5 bg-muted-foreground/40 rounded-full animate-bounce [animation-delay:0ms]" />
                  <span className="w-1.5 h-1.5 bg-muted-foreground/40 rounded-full animate-bounce [animation-delay:150ms]" />
                  <span className="w-1.5 h-1.5 bg-muted-foreground/40 rounded-full animate-bounce [animation-delay:300ms]" />
                </div>
                <span className={cn("text-muted-foreground", compact ? "text-xs" : "text-sm")}>Thinking...</span>
              </div>
            )}
            {/* 流式引用（流结束后展示） */}
          </div>
        </div>
      )}
    </div>
  );
};

export default AIChatMessages;
