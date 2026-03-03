// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/web/src/components/AIChat/ReferenceList.tsx
import { useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { FileText, ExternalLink, Image as ImageIcon, X, ChevronDown, ChevronRight, File } from "lucide-react";
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
  /** RAGFlow chunk 截图 ID */
  imageId?: string;
  /** 原始文档类型 (pdf/docx/pptx 等) */
  docType?: string;
  /** 原始文档名称 */
  documentName?: string;
}

interface ReferenceListProps {
  references: ReferenceItem[];
  className?: string;
  compact?: boolean;
}

// ==================== Chunk 截图组件 ====================

/** chunk 截图缩略图 + 点击放大 */
export function ChunkImage({ imageId, alt, compact }: { imageId: string; alt: string; compact: boolean }) {
  const [isOpen, setIsOpen] = useState(false);
  const [hasError, setHasError] = useState(false);
  const src = `/api/v1/ragflow/image/${imageId}`;

  if (hasError) return null;

  return (
    <>
      {/* 缩略图 */}
      <div
        className={cn(
          "relative cursor-pointer rounded overflow-hidden border border-gray-200 dark:border-gray-600 hover:border-primary/50 transition-colors",
          compact ? "max-w-[120px]" : "max-w-[200px]",
        )}
        onClick={(e) => {
          e.stopPropagation();
          setIsOpen(true);
        }}
      >
        <img
          src={src}
          alt={alt}
          className="w-full h-auto"
          onError={() => setHasError(true)}
          loading="lazy"
        />
        <div className="absolute inset-0 flex items-center justify-center bg-black/0 hover:bg-black/10 transition-colors">
          <ImageIcon className="w-4 h-4 text-white opacity-0 hover:opacity-70 transition-opacity drop-shadow" />
        </div>
      </div>

      {/* 全屏大图弹窗 */}
      {isOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
          onClick={() => setIsOpen(false)}
        >
          <button
            type="button"
            className="absolute top-4 right-4 text-white/80 hover:text-white"
            onClick={() => setIsOpen(false)}
          >
            <X className="w-6 h-6" />
          </button>
          <img
            src={src}
            alt={alt}
            className="max-w-[90vw] max-h-[90vh] object-contain rounded-lg shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      )}
    </>
  );
}

// ==================== 文件类型图标/标签 ====================

/** 根据 docType 返回显示标签 */
export function DocTypeBadge({ docType, compact }: { docType?: string; compact: boolean }) {
  if (!docType) return null;
  const label = docType.toUpperCase();
  return (
    <span
      className={cn(
        "inline-flex items-center rounded px-1 font-mono",
        "bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400",
        compact ? "text-[8px]" : "text-[10px]",
      )}
    >
      {label}
    </span>
  );
}

// ==================== 文档类型图标颜色 ====================

/** 根据文件类型返回对应的图标颜色 */
function getDocTypeColor(docType?: string): string {
  switch (docType?.toLowerCase()) {
    case "pdf":
      return "text-red-500";
    case "docx":
    case "doc":
      return "text-blue-500";
    case "pptx":
    case "ppt":
      return "text-orange-500";
    case "xlsx":
    case "xls":
      return "text-green-500";
    default:
      return "text-gray-500";
  }
}

// ==================== 来源文档聚合 ====================

interface DocAgg {
  documentName: string;
  docType?: string;
  chunkCount: number;
  refIndices: number[];
  isClickable: boolean;
  memoUid?: string;
  attachmentUid?: string;
  type: "memo" | "attachment" | "unknown";
}

/**
 * 引用来源列表组件
 * 重新设计版：
 *   1. Chunk 截图横向滚动（可折叠），带 Fig. N 标签
 *   2. 来源文档卡片 — 按 documentName 去重，显示文件图标 + 文件名
 */
const ReferenceList = ({ references, className, compact = false }: ReferenceListProps) => {
  const navigate = useNavigate();
  const [chunksExpanded, setChunksExpanded] = useState(false);

  // 按 documentName 分组，生成来源文档卡片
  const docAggs = useMemo<DocAgg[]>(() => {
    const map = new Map<string, DocAgg>();
    references.forEach((ref, index) => {
      const key = ref.documentName || ref.title || "unknown";
      const existing = map.get(key);
      if (existing) {
        existing.chunkCount++;
        existing.refIndices.push(index);
      } else {
        map.set(key, {
          documentName: key,
          docType: ref.docType,
          chunkCount: 1,
          refIndices: [index],
          isClickable: (ref.type === "memo" && !!ref.memoUid) || (ref.type === "attachment" && !!ref.attachmentUid),
          memoUid: ref.memoUid,
          attachmentUid: ref.attachmentUid,
          type: ref.type,
        });
      }
    });
    return Array.from(map.values());
  }, [references]);

  // 过滤有截图的 chunk
  const chunksWithImages = useMemo(
    () => references.map((ref, i) => ({ ref, index: i })).filter(({ ref }) => ref.imageId),
    [references],
  );

  if (!references || references.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "border-t border-gray-200 dark:border-gray-700",
        compact ? "mt-2 pt-2" : "mt-3 pt-3",
        className,
      )}
    >
      {/* ==================== Chunk 截图横向展示（可折叠） ==================== */}
      {chunksWithImages.length > 0 && (
        <div className="mb-2">
          {/* 折叠按钮 */}
          <button
            type="button"
            onClick={() => setChunksExpanded(!chunksExpanded)}
            className={cn(
              "flex items-center gap-1 text-muted-foreground w-full text-left mb-1.5",
              compact ? "text-[10px]" : "text-xs",
            )}
          >
            {chunksExpanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
            <ImageIcon className={compact ? "w-3 h-3" : "w-3.5 h-3.5"} />
            <span>引用片段 ({chunksWithImages.length})</span>
          </button>

          {/* 横向滚动截图列表 */}
          {chunksExpanded && (
            <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-thin">
              {chunksWithImages.map(({ ref, index }) => (
                <div
                  key={`chunk-${index}`}
                  className="shrink-0 flex flex-col items-center gap-1"
                >
                  <ChunkImage
                    imageId={ref.imageId!}
                    alt={ref.title || `chunk ${index + 1}`}
                    compact={compact}
                  />
                  <span
                    className={cn(
                      "text-primary font-medium bg-primary/10 rounded-full px-2 py-0.5",
                      compact ? "text-[10px]" : "text-xs",
                    )}
                  >
                    Fig. {index + 1}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ==================== 来源文档卡片 ==================== */}
      <div className={cn("flex items-center gap-1.5 text-muted-foreground mb-2", compact ? "text-[10px]" : "text-xs")}>
        <FileText className={compact ? "w-3 h-3" : "w-3.5 h-3.5"} />
        <span>来源文档 ({docAggs.length})</span>
      </div>

      <div className="flex flex-wrap gap-2">
        {docAggs.map((doc) => (
          <div
            key={doc.documentName}
            onClick={() => {
              if (!doc.isClickable) return;
              if (doc.type === "memo" && doc.memoUid) {
                navigate(`/memos/${doc.memoUid}`);
              } else if (doc.type === "attachment" && doc.attachmentUid) {
                window.open(`/file/attachments/${doc.attachmentUid}/${doc.documentName}`, "_blank");
              }
            }}
            className={cn(
              "group flex items-center gap-2 rounded-lg border transition-colors",
              "bg-gray-50 dark:bg-gray-800/50 border-gray-200 dark:border-gray-700",
              doc.isClickable && "cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 hover:border-primary/30",
              compact ? "px-2 py-1.5 max-w-[180px]" : "px-3 py-2 max-w-[240px]",
            )}
          >
            {/* 文件图标 */}
            <File className={cn("shrink-0", getDocTypeColor(doc.docType), compact ? "w-4 h-4" : "w-5 h-5")} />
            {/* 文件名 */}
            <div className="flex-1 min-w-0">
              <div className={cn("font-medium text-gray-700 dark:text-gray-300 truncate", compact ? "text-[11px]" : "text-xs")}>
                {doc.documentName}
              </div>
            </div>
            {/* 跳转图标 */}
            {doc.isClickable && (
              <ExternalLink
                className={cn(
                  "text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity shrink-0",
                  compact ? "w-3 h-3" : "w-3.5 h-3.5",
                )}
              />
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default ReferenceList;
