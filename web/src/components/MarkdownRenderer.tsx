import { memo, useMemo } from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { CodeBlock } from "./MemoContent/CodeBlock";
import { SANITIZE_SCHEMA } from "./MemoContent/constants";
import type { ReferenceItem } from "./AIChat/ReferenceList";
import InlineReference from "./AIChat/InlineReference";

// ==================== [ID:N] → <cite> 预处理 ====================

/** 匹配 RAGFlow 行内引用标记 [ID:N]，N 为 0-based 整数 */
const CITATION_REG = /\[ID:(\d+)\]/g;

/**
 * 将 Markdown 文本中的 [ID:N] 替换为 <cite data-ref="N"></cite>。
 * 只在非代码块区域替换（简单策略：跳过 ``` 围栏内的内容）。
 */
function preprocessCitations(text: string): string {
  if (!CITATION_REG.test(text)) return text;
  // 重置 lastIndex（全局正则的坑）
  CITATION_REG.lastIndex = 0;

  // 按代码围栏分割，奇数段是代码块内容，偶数段是普通文本
  const parts = text.split(/(```[\s\S]*?```)/g);
  return parts
    .map((part, i) => {
      // 奇数索引 = 代码块，不替换
      if (i % 2 === 1) return part;
      return part.replace(CITATION_REG, (_match, id) => {
        return `<cite data-ref="${id}"></cite>`;
      });
    })
    .join("");
}

// ==================== 组件 ====================

interface MarkdownRendererProps {
  content: string;
  className?: string;
  /** AI 引用数据，传入后启用行内引用标记渲染 */
  references?: ReferenceItem[];
}

const MarkdownRenderer = ({ content, className, references }: MarkdownRendererProps) => {
  // 仅在有 references 时执行预处理（普通 Memo 渲染不受影响）
  const processedContent = useMemo(
    () => (references && references.length > 0 ? preprocessCitations(content) : content),
    [content, references],
  );

  // 构建 components 对象（仅在有 references 时注册 cite 组件）
  const components = useMemo(() => {
    const base: Record<string, React.ComponentType<any>> = {
      pre: CodeBlock,
      a: ({ href, children, ...aProps }: any) => (
        <a href={href} target="_blank" rel="noopener noreferrer" {...aProps}>
          {children}
        </a>
      ),
    };

    if (references && references.length > 0) {
      base.cite = ({ node, ...props }: any) => {
        const dataRef = props["data-ref"] ?? node?.properties?.["dataRef"];
        if (dataRef == null) return <cite {...props} />;
        const chunkIndex = parseInt(String(dataRef), 10);
        if (isNaN(chunkIndex)) return <cite {...props} />;
        return <InlineReference chunkIndex={chunkIndex} reference={references[chunkIndex]} />;
      };
    }

    return base;
  }, [references]);

  return (
    <div className={className}>
      <ReactMarkdown
        remarkPlugins={[remarkMath, remarkGfm, remarkBreaks]}
        rehypePlugins={[rehypeRaw, rehypeKatex, [rehypeSanitize, SANITIZE_SCHEMA]]}
        components={components}
      >
        {processedContent}
      </ReactMarkdown>
    </div>
  );
};

export default memo(MarkdownRenderer);
