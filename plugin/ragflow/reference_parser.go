package ragflow

import (
	"strings"
)

// ==================== 引用解析器 ====================
// 职责：将 RAGFlow OpenAI 兼容 API 返回的 chunks_format 引用
// 转换为 Knowtree 可用的结构化引用（含 Memo UID / Attachment UID）
//
// 命名规范（与 P1 同步编排器一致）:
//   Memo:       "memo_{memo_uid}.txt"
//   Attachment: "attachment_{att_uid}_{original_filename}"

// ==================== 解析后的引用类型 ====================

// ParsedReference 解析后的引用信息（面向前端和 DB 存储）
type ParsedReference struct {
	// MemoUID Memo 的唯一标识（type=memo 时非空）
	MemoUID string `json:"memo_uid,omitempty"`
	// AttachmentUID 附件的唯一标识（type=attachment 时非空）
	AttachmentUID string `json:"attachment_uid,omitempty"`
	// Type 引用类型: "memo" | "attachment" | "unknown"
	Type string `json:"type"`
	// Title 显示标题（待 Service 层从 DB 查询填充）
	Title string `json:"title"`
	// ContentSnippet 匹配的内容片段（截取前 200 字符）
	ContentSnippet string `json:"content_snippet"`
	// Similarity 相似度得分
	Similarity float64 `json:"similarity"`
	// DocumentName RAGFlow 原始文档名（调试用）
	DocumentName string `json:"document_name"`

	// === RAGFlow chunk 可视化字段 ===

	// ImageID RAGFlow chunk 截图 ID（格式 "{kb_id}-{chunk_id}"，通过 /v1/document/image/{id} 获取）
	// 空字符串表示该 chunk 无截图（纯文本文件等）
	ImageID string `json:"image_id,omitempty"`
	// Positions chunk 在原始文档中的页面坐标 [[x0,y0,x1,y1], ...]
	Positions [][]int `json:"positions,omitempty"`
	// DocType 原始文档类型（"pdf"/"docx"/"pptx"/"xlsx" 等）
	DocType string `json:"doc_type,omitempty"`
}

// 引用类型常量
const (
	ReferenceTypeMemo       = "memo"
	ReferenceTypeAttachment = "attachment"
	ReferenceTypeUnknown    = "unknown"
)

// 文档名前缀（与 P1 同步编排器的命名规范对齐）
const (
	docPrefixMemo       = "memo_"
	docPrefixAttachment = "attachment_"
	docSuffixTxt        = ".txt"
)

// 内容片段最大长度
const maxSnippetLen = 200

// ==================== 核心解析函数 ====================

// ParseReferences 将 RAGFlow OpenAI API 返回的引用列表转换为 Knowtree 引用
// 解析规则：
//   - "memo_{uid}.txt"                    → type=memo,       MemoUID={uid}
//   - "attachment_{uid}_{filename}"        → type=attachment,  AttachmentUID={uid}
//   - 其他                                 → type=unknown
//
// 解析失败时静默降级为 unknown 类型，不返回错误
func ParseReferences(refs []OpenAIReference) []ParsedReference {
	if len(refs) == 0 {
		return nil
	}

	parsed := make([]ParsedReference, 0, len(refs))
	for _, ref := range refs {
		parsed = append(parsed, parseOneReference(ref))
	}
	return parsed
}

// parseOneReference 解析单条引用
func parseOneReference(ref OpenAIReference) ParsedReference {
	result := ParsedReference{
		Similarity:     ref.Similarity,
		DocumentName:   ref.DocumentName,
		ContentSnippet: truncateSnippet(ref.Content, maxSnippetLen),
		// 透传 RAGFlow chunk 可视化字段
		ImageID:   ref.ImageID,
		Positions: ref.Positions,
		DocType:   ref.DocType,
	}

	name := ref.DocumentName

	switch {
	case strings.HasPrefix(name, docPrefixMemo):
		// memo_{uid}.txt → 提取 uid
		uid := strings.TrimPrefix(name, docPrefixMemo)
		uid = strings.TrimSuffix(uid, docSuffixTxt)
		if uid != "" {
			result.Type = ReferenceTypeMemo
			result.MemoUID = uid
			return result
		}

	case strings.HasPrefix(name, docPrefixAttachment):
		// attachment_{uid}_{filename} → 提取 uid
		uid := extractAttachmentUID(name)
		if uid != "" {
			result.Type = ReferenceTypeAttachment
			result.AttachmentUID = uid
			return result
		}
	}

	// 无法识别的格式，降级为 unknown
	result.Type = ReferenceTypeUnknown
	return result
}

// extractAttachmentUID 从附件文档名中提取 UID
// 格式: "attachment_{uid}_{original_filename}"
// 规则: 去掉 "attachment_" 前缀后，取第一个 "_" 之前的部分作为 UID
//
// 示例:
//
//	"attachment_att_xyz789_report.pdf" → "att_xyz789"
//
// 注意: UID 本身可能含下划线（如 "att_xyz789"），
// 因此需要匹配 "att_" 前缀来确定 UID 的边界
func extractAttachmentUID(name string) string {
	// 去掉 "attachment_" 前缀
	rest := strings.TrimPrefix(name, docPrefixAttachment)
	if rest == "" {
		return ""
	}

	// 附件 UID 以 "att_" 开头，找到 "att_" 后的第一个 "_" 作为 UID 结束位置
	if strings.HasPrefix(rest, "att_") {
		// "att_xyz789_report.pdf" → 在 "att_" 之后找第一个 "_"
		afterPrefix := rest[4:] // 跳过 "att_"
		idx := strings.Index(afterPrefix, "_")
		if idx > 0 {
			return rest[:4+idx] // "att_xyz789"
		}
		// 没有后续的 "_"，整个 rest 就是 UID（无原始文件名的边缘情况）
		return rest
	}

	// UID 不以 "att_" 开头，取第一个 "_" 之前的部分
	idx := strings.Index(rest, "_")
	if idx > 0 {
		return rest[:idx]
	}

	// 没有 "_"，整个 rest 就是 UID
	return rest
}

// truncateSnippet 截取内容片段，按 rune 截取以正确处理中文
func truncateSnippet(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
