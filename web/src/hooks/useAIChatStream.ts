// ==================== SSE 流式消费 Hook ====================
// 消费后端 POST /api/v1/ai/conversations/:id/messages/stream
// 支持 4 种事件类型: content / reasoning / done / error

import { useState, useCallback, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { getAccessToken } from "@/auth-state";
import { aiKeys } from "./useAIQueries";

// ==================== 类型定义 ====================

/** SSE 事件载荷（与后端 SSEEvent 结构体对齐） */
interface SSEEvent {
  type: "content" | "reasoning" | "done" | "error";
  content?: string;
  reasoning_content?: string;
  message_id?: string;
  references_json?: string;
  token_usage_json?: string;
  error?: string;
}

/** 引用来源（从 references_json 解析，与后端 ParsedReference 对齐） */
export interface StreamReference {
  memo_uid?: string;
  attachment_uid?: string;
  type: "memo" | "attachment" | "unknown";
  title: string;
  content_snippet: string;
  similarity: number;
  document_name: string;
  /** RAGFlow chunk 截图 ID（通过 /api/v1/ragflow/image/{id} 获取） */
  image_id?: string;
  /** chunk 在文档中的页面坐标 */
  positions?: number[][];
  /** 原始文档类型 (pdf/docx/pptx 等) */
  doc_type?: string;
}

/** Hook 返回的流式状态 */
export interface StreamState {
  /** 正在进行流式传输 */
  isStreaming: boolean;
  /** 累积的增量文本 */
  streamingContent: string;
  /** 累积的思考链文本 */
  reasoningContent: string;
  /** 引用来源列表（流结束后填充） */
  references: StreamReference[];
  /** 助手消息 UID（流结束后填充） */
  messageId: string;
  /** 错误信息 */
  error: string | null;
}

// ==================== Hook 实现 ====================

export function useAIChatStream() {
  const queryClient = useQueryClient();
  const abortRef = useRef<AbortController | null>(null);

  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState("");
  const [reasoningContent, setReasoningContent] = useState("");
  const [references, setReferences] = useState<StreamReference[]>([]);
  const [messageId, setMessageId] = useState("");
  const [error, setError] = useState<string | null>(null);

  /** 重置所有流式状态 */
  const reset = useCallback(() => {
    setStreamingContent("");
    setReasoningContent("");
    setReferences([]);
    setMessageId("");
    setError(null);
  }, []);

  /** 中止当前流式请求 */
  const abort = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort();
      abortRef.current = null;
    }
    setIsStreaming(false);
  }, []);

  /** 发送消息并消费 SSE 流 */
  const sendStreamMessage = useCallback(
    async (conversationUid: string, content: string) => {
      // 中止之前的请求
      abort();
      reset();
      setIsStreaming(true);

      const controller = new AbortController();
      abortRef.current = controller;

      try {
        const token = getAccessToken();
        const response = await fetch(
          `/api/v1/ai/conversations/${conversationUid}/messages/stream`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              ...(token ? { Authorization: `Bearer ${token}` } : {}),
            },
            body: JSON.stringify({ content }),
            signal: controller.signal,
          },
        );

        if (!response.ok) {
          const text = await response.text().catch(() => "Unknown error");
          throw new Error(`HTTP ${response.status}: ${text}`);
        }

        if (!response.body) {
          throw new Error("Response body is empty");
        }

        // 消费 ReadableStream
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });

          // 按行解析 SSE 事件
          const lines = buffer.split("\n");
          // 保留最后一个可能不完整的行
          buffer = lines.pop() || "";

          for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed || !trimmed.startsWith("data: ")) continue;

            const jsonStr = trimmed.slice(6); // 去掉 "data: " 前缀
            if (jsonStr === "[DONE]") continue;

            try {
              const event: SSEEvent = JSON.parse(jsonStr);

              switch (event.type) {
                case "content":
                  if (event.content) {
                    setStreamingContent((prev) => prev + event.content);
                  }
                  break;

                case "reasoning":
                  if (event.reasoning_content) {
                    setReasoningContent((prev) => prev + event.reasoning_content);
                  }
                  break;

                case "done":
                  if (event.message_id) {
                    setMessageId(event.message_id);
                  }
                  if (event.references_json) {
                    try {
                      const refs: StreamReference[] = JSON.parse(event.references_json);
                      setReferences(refs);
                    } catch {
                      // 引用解析失败时静默降级
                    }
                  }
                  break;

                case "error":
                  setError(event.error || "Unknown error");
                  break;
              }
            } catch {
              // JSON 解析失败，跳过该行
            }
          }
        }
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") {
          // 用户主动中止，不设置错误
          return;
        }
        setError(err instanceof Error ? err.message : "Stream failed");
      } finally {
        setIsStreaming(false);
        abortRef.current = null;

        // 流结束后刷新对话和消息缓存
        queryClient.invalidateQueries({ queryKey: aiKeys.conversations() });
        if (conversationUid) {
          queryClient.invalidateQueries({ queryKey: aiKeys.conversation(conversationUid) });
          queryClient.invalidateQueries({ queryKey: aiKeys.messages(conversationUid) });
        }
      }
    },
    [abort, reset, queryClient],
  );

  return {
    sendStreamMessage,
    abort,
    reset,
    isStreaming,
    streamingContent,
    reasoningContent,
    references,
    messageId,
    error,
  } as const;
}
