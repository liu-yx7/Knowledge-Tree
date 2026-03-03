//
// 行内引用标记组件 — 在 AI 回答文本中渲染 "Fig. N" 悬浮引用
// 对应 RAGFlow 原生 UI 中 [ID:N] → Fig. N 的展示效果：
//   - 在句末显示 "Fig. 1" 圆角标签
//   - 鼠标悬浮时弹出 Popover，展示对应 chunk 的内容/截图
//
// 与 MarkdownRenderer 配合使用：
//   Markdown 预处理将 [ID:N] 替换为 <cite data-ref="N"></cite>
//   ReactMarkdown 的 cite 自定义组件渲染本组件

import { useState, useRef, useCallback } from "react";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { FileText, Paperclip } from "lucide-react";
import { cn } from "@/lib/utils";
import type { ReferenceItem } from "./ReferenceList";
import { DocTypeBadge } from "./ReferenceList";

interface InlineReferenceProps {
  /** 0-based chunk 索引 */
  chunkIndex: number;
  /** 对应的引用数据（可能为 undefined，索引越界时降级） */
  reference?: ReferenceItem;
}

/**
 * 行内引用标记 — "Fig. N" 悬浮标签
 *
 * 渲染为行内 span，鼠标悬浮 300ms 后弹出 Popover 展示 chunk 详情。
 * 若无对应引用数据（reference 为 undefined），只显示标签不弹出。
 */
export default function InlineReference({ chunkIndex, reference }: InlineReferenceProps) {
  const [open, setOpen] = useState(false);
  const enterTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const leaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleMouseEnter = useCallback(() => {
    // 取消延迟关闭
    if (leaveTimerRef.current) {
      clearTimeout(leaveTimerRef.current);
      leaveTimerRef.current = null;
    }
    // 延迟 200ms 打开，避免快速划过时闪烁
    enterTimerRef.current = setTimeout(() => setOpen(true), 200);
  }, []);

  const handleMouseLeave = useCallback(() => {
    // 取消延迟打开
    if (enterTimerRef.current) {
      clearTimeout(enterTimerRef.current);
      enterTimerRef.current = null;
    }
    // 延迟 150ms 关闭，给用户移动到 popover 内容区的缓冲时间
    leaveTimerRef.current = setTimeout(() => setOpen(false), 150);
  }, []);

  const displayIndex = chunkIndex + 1; // 1-based display

  // 无引用数据 → 仅渲染标签
  if (!reference) {
    return (
      <span
        className="inline-flex items-center text-xs text-muted-foreground bg-muted/60 rounded-2xl px-1.5 py-0.5 mx-0.5 font-medium cursor-default select-none"
        title={`Fig. ${displayIndex}`}
      >
        Fig.&nbsp;{displayIndex}
      </span>
    );
  }

  const Icon = reference.type === "attachment" ? Paperclip : FileText;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <span
          className="inline-flex items-center text-xs text-primary bg-primary/10 hover:bg-primary/20 rounded-2xl px-1.5 py-0.5 mx-0.5 font-medium cursor-pointer select-none transition-colors"
          onMouseEnter={handleMouseEnter}
          onMouseLeave={handleMouseLeave}
        >
          Fig.&nbsp;{displayIndex}
        </span>
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        sideOffset={6}
        className="max-w-sm p-3 space-y-2"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        // 阻止 popover 内点击导致关闭
        onOpenAutoFocus={(e) => e.preventDefault()}
      >
        {/* 标题行 */}
        <div className="flex items-center gap-1.5">
          <Icon className="w-3.5 h-3.5 shrink-0 text-muted-foreground" />
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300 truncate flex-1">
            {reference.title}
          </span>
          <DocTypeBadge docType={reference.docType} compact={false} />
          <span className="text-[10px] text-gray-400 dark:text-gray-500 shrink-0">
            {(reference.similarity * 100).toFixed(0)}%
          </span>
        </div>

        {/* 来源文件 */}
        {reference.documentName && (
          <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
            <FileText className="w-3 h-3 shrink-0" />
            <span className="truncate">{reference.documentName}</span>
          </div>
        )}

        {/* Chunk 截图（PDF 等有 imageId 时展示） */}
        {reference.imageId && (
          <div className={cn("rounded overflow-hidden border border-gray-200 dark:border-gray-600 max-w-[300px]")}>
            <img
              src={`/api/v1/ragflow/image/${reference.imageId}`}
              alt={reference.title || "chunk"}
              className="w-full h-auto max-h-[200px] object-contain"
              loading="lazy"
            />
          </div>
        )}

        {/* 内容片段 */}
        {reference.contentSnippet && (
          <p className="text-xs text-gray-500 dark:text-gray-400 line-clamp-4 leading-relaxed">
            {reference.contentSnippet}
          </p>
        )}
      </PopoverContent>
    </Popover>
  );
}
