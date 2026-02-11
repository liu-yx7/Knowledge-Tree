import { useState } from "react";
import { Search, Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useSemanticSearch, type Reference } from "@/hooks/useRAGFlowQueries";
import ReferenceList, { type ReferenceItem } from "./ReferenceList";

interface SemanticSearchPanelProps {
  className?: string;
  onClose?: () => void;
  compact?: boolean;
}

/** 语义搜索 Reference → 通用 ReferenceItem 转换 */
function toReferenceItem(ref: Reference): ReferenceItem {
  return {
    memoUid: ref.memoUid,
    type: "memo",
    title: ref.title,
    contentSnippet: ref.contentSnippet,
    similarity: ref.similarityScore,
  };
}

/**
 * 语义搜索面板组件
 * 使用 RAGFlow 对用户的知识库进行语义检索
 */
const SemanticSearchPanel = ({ className, onClose, compact = false }: SemanticSearchPanelProps) => {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Reference[]>([]);
  const [hasSearched, setHasSearched] = useState(false);

  const semanticSearch = useSemanticSearch();

  const handleSearch = async () => {
    if (!query.trim()) return;

    try {
      const searchResults = await semanticSearch.mutateAsync({
        query: query.trim(),
        topK: 6,
        similarityThreshold: 0.3,
      });
      setResults(searchResults);
      setHasSearched(true);
    } catch (error) {
      console.error("语义搜索失败:", error);
      setResults([]);
      setHasSearched(true);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleSearch();
    }
  };

  const handleClear = () => {
    setQuery("");
    setResults([]);
    setHasSearched(false);
  };

  return (
    <div className={cn(
      "flex flex-col border rounded-lg bg-background",
      compact ? "p-2" : "p-4",
      className
    )}>
      {/* 标题栏 */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Search className={compact ? "w-4 h-4" : "w-5 h-5"} />
          <span className={cn(
            "font-medium",
            compact ? "text-sm" : "text-base"
          )}>
            语义搜索
          </span>
        </div>
        {onClose && (
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={onClose}
          >
            <X className="w-4 h-4" />
          </Button>
        )}
      </div>

      {/* 搜索框 */}
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Input
            placeholder="搜索你的知识库..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            className={compact ? "h-8 text-sm" : "h-9"}
            disabled={semanticSearch.isPending}
          />
          {query && !semanticSearch.isPending && (
            <Button
              variant="ghost"
              size="icon"
              className="absolute right-1 top-1/2 -translate-y-1/2 h-6 w-6"
              onClick={handleClear}
            >
              <X className="w-3 h-3" />
            </Button>
          )}
        </div>
        <Button
          onClick={handleSearch}
          disabled={!query.trim() || semanticSearch.isPending}
          size={compact ? "sm" : "default"}
          className={compact ? "h-8" : "h-9"}
        >
          {semanticSearch.isPending ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <Search className="w-4 h-4" />
          )}
        </Button>
      </div>

      {/* 搜索结果 */}
      <div className="mt-3 flex-1 overflow-auto">
        {semanticSearch.isPending ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        ) : hasSearched ? (
          results.length > 0 ? (
            <ReferenceList references={results.map(toReferenceItem)} compact={compact} className="border-t-0 mt-0 pt-0" />
          ) : (
            <div className={cn(
              "text-center text-muted-foreground py-8",
              compact ? "text-xs" : "text-sm"
            )}>
              未找到相关内容
            </div>
          )
        ) : (
          <div className={cn(
            "text-center text-muted-foreground py-8",
            compact ? "text-xs" : "text-sm"
          )}>
            输入关键词搜索你的笔记
          </div>
        )}
      </div>
    </div>
  );
};

export default SemanticSearchPanel;
