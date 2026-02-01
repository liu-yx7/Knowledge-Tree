// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/web/src/components/AIChat/ReferenceList.tsx
import { useNavigate } from "react-router-dom";
import { FileText, ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";
import type { Reference } from "@/hooks/useRAGFlowQueries";

interface ReferenceListProps {
  references: Reference[];
  className?: string;
  compact?: boolean;
}

/**
 * 引用来源列表组件
 * 展示 AI 回答中引用的 Memo 来源，支持点击跳转
 */
const ReferenceList = ({ references, className, compact = false }: ReferenceListProps) => {
  const navigate = useNavigate();

  if (!references || references.length === 0) {
    return null;
  }

  const handleReferenceClick = (memoUid: string) => {
    // 跳转到对应的 Memo 详情页
    navigate(`/m/${memoUid}`);
  };

  return (
    <div
      className={cn(
        "border-t border-gray-200 dark:border-gray-700",
        compact ? "mt-2 pt-2" : "mt-3 pt-3",
        className
      )}
    >
      {/* 标题 */}
      <div className={cn(
        "flex items-center gap-1.5 text-muted-foreground mb-2",
        compact ? "text-[10px]" : "text-xs"
      )}>
        <FileText className={compact ? "w-3 h-3" : "w-3.5 h-3.5"} />
        <span>引用来源 ({references.length})</span>
      </div>

      {/* 引用列表 */}
      <div className={cn("space-y-1.5", compact && "space-y-1")}>
        {references.map((ref, index) => (
          <div
            key={`${ref.memoUid}-${index}`}
            onClick={() => handleReferenceClick(ref.memoUid)}
            className={cn(
              "group rounded-md cursor-pointer transition-colors",
              "bg-gray-50 dark:bg-gray-800/50",
              "hover:bg-gray-100 dark:hover:bg-gray-800",
              compact ? "p-1.5" : "p-2"
            )}
          >
            {/* 标题行 */}
            <div className="flex items-center justify-between gap-2">
              <span className={cn(
                "font-medium text-gray-700 dark:text-gray-300 truncate flex-1",
                compact ? "text-xs" : "text-sm"
              )}>
                {ref.title}
              </span>
              <div className="flex items-center gap-2 shrink-0">
                {/* 相似度分数 */}
                <span className={cn(
                  "text-gray-400 dark:text-gray-500",
                  compact ? "text-[10px]" : "text-xs"
                )}>
                  {(ref.similarityScore * 100).toFixed(0)}%
                </span>
                {/* 跳转图标 */}
                <ExternalLink className={cn(
                  "text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity",
                  compact ? "w-3 h-3" : "w-3.5 h-3.5"
                )} />
              </div>
            </div>

            {/* 内容片段 */}
            {!compact && ref.contentSnippet && (
              <p className="text-xs text-gray-500 dark:text-gray-400 line-clamp-2 mt-1">
                {ref.contentSnippet}
              </p>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default ReferenceList;
