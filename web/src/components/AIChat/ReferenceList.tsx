// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/web/src/components/AIChat/ReferenceList.tsx
import { useNavigate } from "react-router-dom";
import { FileText, ExternalLink, Paperclip } from "lucide-react";
import { cn } from "@/lib/utils";

// ==================== 通用引用类型 ====================

/** 统一引用项（兼容 SSE 流式引用和语义搜索引用） */
export interface ReferenceItem {
  /** Memo UID（memo 类型时非空） */
  memoUid?: string;
  /** Attachment UID（attachment 类型时非空） */
  attachmentUid?: string;
  /** 引用类型 */
  type: "memo" | "attachment" | "unknown";
  /** 显示标题 */
  title: string;
  /** 内容片段 */
  contentSnippet?: string;
  /** 相似度分数 (0-1) */
  similarity: number;
}

interface ReferenceListProps {
  references: ReferenceItem[];
  className?: string;
  compact?: boolean;
}

/**
 * 引用来源列表组件
 * 展示 AI 回答中引用的 Memo/Attachment 来源，支持点击跳转
 */
const ReferenceList = ({ references, className, compact = false }: ReferenceListProps) => {
  const navigate = useNavigate();

  if (!references || references.length === 0) {
    return null;
  }

  const handleClick = (ref: ReferenceItem) => {
    if (ref.type === "memo" && ref.memoUid) {
      navigate(`/m/${ref.memoUid}`);
    }
    // attachment 类型暂不支持跳转
  };

  return (
    <div
      className={cn(
        "border-t border-gray-200 dark:border-gray-700",
        compact ? "mt-2 pt-2" : "mt-3 pt-3",
        className,
      )}
    >
      {/* 标题 */}
      <div className={cn("flex items-center gap-1.5 text-muted-foreground mb-2", compact ? "text-[10px]" : "text-xs")}>
        <FileText className={compact ? "w-3 h-3" : "w-3.5 h-3.5"} />
        <span>
          引用来源 ({references.length})
        </span>
      </div>

      {/* 引用列表 */}
      <div className={cn("space-y-1.5", compact && "space-y-1")}>
        {references.map((ref, index) => {
          const isClickable = ref.type === "memo" && !!ref.memoUid;
          const Icon = ref.type === "attachment" ? Paperclip : FileText;

          return (
            <div
              key={`${ref.memoUid || ref.attachmentUid || "unknown"}-${index}`}
              onClick={() => isClickable && handleClick(ref)}
              className={cn(
                "group rounded-md transition-colors",
                "bg-gray-50 dark:bg-gray-800/50",
                isClickable && "cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800",
                compact ? "p-1.5" : "p-2",
              )}
            >
              {/* 标题行 */}
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-1.5 flex-1 min-w-0">
                  <Icon className={cn("shrink-0 text-muted-foreground", compact ? "w-3 h-3" : "w-3.5 h-3.5")} />
                  <span
                    className={cn(
                      "font-medium text-gray-700 dark:text-gray-300 truncate",
                      compact ? "text-xs" : "text-sm",
                    )}
                  >
                    {ref.title}
                  </span>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  {/* 相似度分数 */}
                  <span className={cn("text-gray-400 dark:text-gray-500", compact ? "text-[10px]" : "text-xs")}>
                    {(ref.similarity * 100).toFixed(0)}%
                  </span>
                  {/* 跳转图标 */}
                  {isClickable && (
                    <ExternalLink
                      className={cn(
                        "text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity",
                        compact ? "w-3 h-3" : "w-3.5 h-3.5",
                      )}
                    />
                  )}
                </div>
              </div>

              {/* 内容片段 */}
              {!compact && ref.contentSnippet && (
                <p className="text-xs text-gray-500 dark:text-gray-400 line-clamp-2 mt-1 ml-5">
                  {ref.contentSnippet}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default ReferenceList;
