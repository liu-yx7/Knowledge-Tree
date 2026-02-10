package ragflow

import (
	"strings"
	"testing"
)

// ==================== ParseReferences 整体测试 ====================

func TestParseReferences_MemoReference(t *testing.T) {
	refs := []OpenAIReference{
		{
			ID:           "chunk-001",
			Content:      "Go 的 goroutine 是轻量级线程",
			DocumentID:   "doc-001",
			DocumentName: "memo_m_abc123def456.txt",
			DatasetID:    "ds-001",
			Similarity:   0.85,
		},
	}

	parsed := ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("解析结果数量不正确: %d", len(parsed))
	}

	p := parsed[0]
	if p.Type != ReferenceTypeMemo {
		t.Errorf("类型不正确: %s", p.Type)
	}
	if p.MemoUID != "m_abc123def456" {
		t.Errorf("MemoUID 不正确: %s", p.MemoUID)
	}
	if p.AttachmentUID != "" {
		t.Errorf("AttachmentUID 应为空: %s", p.AttachmentUID)
	}
	if p.Similarity != 0.85 {
		t.Errorf("Similarity 不正确: %f", p.Similarity)
	}
	if p.DocumentName != "memo_m_abc123def456.txt" {
		t.Errorf("DocumentName 不正确: %s", p.DocumentName)
	}
	if p.ContentSnippet != "Go 的 goroutine 是轻量级线程" {
		t.Errorf("ContentSnippet 不正确: %s", p.ContentSnippet)
	}
}

func TestParseReferences_AttachmentReference(t *testing.T) {
	refs := []OpenAIReference{
		{
			ID:           "chunk-002",
			Content:      "CSP 模型与 channel 通信",
			DocumentID:   "doc-002",
			DocumentName: "attachment_att_xyz789_report.pdf",
			DatasetID:    "ds-001",
			Similarity:   0.72,
		},
	}

	parsed := ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("解析结果数量不正确: %d", len(parsed))
	}

	p := parsed[0]
	if p.Type != ReferenceTypeAttachment {
		t.Errorf("类型不正确: %s", p.Type)
	}
	if p.AttachmentUID != "att_xyz789" {
		t.Errorf("AttachmentUID 不正确: %s", p.AttachmentUID)
	}
	if p.MemoUID != "" {
		t.Errorf("MemoUID 应为空: %s", p.MemoUID)
	}
	if p.Similarity != 0.72 {
		t.Errorf("Similarity 不正确: %f", p.Similarity)
	}
}

func TestParseReferences_UnknownFormat(t *testing.T) {
	refs := []OpenAIReference{
		{
			ID:           "chunk-003",
			Content:      "一些内容",
			DocumentName: "random_document.pdf",
			Similarity:   0.5,
		},
	}

	parsed := ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("解析结果数量不正确: %d", len(parsed))
	}

	p := parsed[0]
	if p.Type != ReferenceTypeUnknown {
		t.Errorf("类型应为 unknown: %s", p.Type)
	}
	if p.MemoUID != "" {
		t.Errorf("MemoUID 应为空: %s", p.MemoUID)
	}
	if p.AttachmentUID != "" {
		t.Errorf("AttachmentUID 应为空: %s", p.AttachmentUID)
	}
}

func TestParseReferences_MultipleRefs(t *testing.T) {
	refs := []OpenAIReference{
		{DocumentName: "memo_m_first.txt", Similarity: 0.9},
		{DocumentName: "attachment_att_second_file.pdf", Similarity: 0.8},
		{DocumentName: "unknown_doc.txt", Similarity: 0.7},
	}

	parsed := ParseReferences(refs)

	if len(parsed) != 3 {
		t.Fatalf("解析结果数量不正确: %d", len(parsed))
	}

	if parsed[0].Type != ReferenceTypeMemo || parsed[0].MemoUID != "m_first" {
		t.Errorf("第一个引用解析不正确: type=%s, uid=%s", parsed[0].Type, parsed[0].MemoUID)
	}
	if parsed[1].Type != ReferenceTypeAttachment || parsed[1].AttachmentUID != "att_second" {
		t.Errorf("第二个引用解析不正确: type=%s, uid=%s", parsed[1].Type, parsed[1].AttachmentUID)
	}
	if parsed[2].Type != ReferenceTypeUnknown {
		t.Errorf("第三个引用应为 unknown: %s", parsed[2].Type)
	}
}

func TestParseReferences_EmptyInput(t *testing.T) {
	// nil 输入
	parsed := ParseReferences(nil)
	if parsed != nil {
		t.Errorf("nil 输入应返回 nil: %v", parsed)
	}

	// 空切片
	parsed = ParseReferences([]OpenAIReference{})
	if parsed != nil {
		t.Errorf("空切片应返回 nil: %v", parsed)
	}
}

// ==================== Memo 引用解析边界情况 ====================

func TestParseReferences_MemoWithoutTxtSuffix(t *testing.T) {
	// memo_ 前缀但无 .txt 后缀
	refs := []OpenAIReference{
		{DocumentName: "memo_m_nosuffix", Similarity: 0.5},
	}
	parsed := ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("数量不正确: %d", len(parsed))
	}
	// 无 .txt 后缀，TrimSuffix 不起作用，但 UID 仍然可提取
	if parsed[0].Type != ReferenceTypeMemo {
		t.Errorf("类型不正确: %s", parsed[0].Type)
	}
	if parsed[0].MemoUID != "m_nosuffix" {
		t.Errorf("MemoUID 不正确: %s", parsed[0].MemoUID)
	}
}

func TestParseReferences_MemoEmptyUID(t *testing.T) {
	// "memo_.txt" → UID 为空 → 降级为 unknown
	refs := []OpenAIReference{
		{DocumentName: "memo_.txt", Similarity: 0.5},
	}
	parsed := ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("数量不正确: %d", len(parsed))
	}
	if parsed[0].Type != ReferenceTypeUnknown {
		t.Errorf("空 UID 应降级为 unknown: %s", parsed[0].Type)
	}
}

func TestParseReferences_MemoOnlyPrefix(t *testing.T) {
	// "memo_" → 去前缀后为空 → TrimSuffix 后仍为空 → 降级为 unknown
	refs := []OpenAIReference{
		{DocumentName: "memo_", Similarity: 0.5},
	}
	parsed := ParseReferences(refs)

	if parsed[0].Type != ReferenceTypeUnknown {
		t.Errorf("仅前缀应降级为 unknown: %s", parsed[0].Type)
	}
}

// ==================== Attachment 引用解析边界情况 ====================

func TestParseReferences_AttachmentWithComplexUID(t *testing.T) {
	tests := []struct {
		name        string
		docName     string
		expectedUID string
	}{
		{
			"标准格式",
			"attachment_att_xyz789_report.pdf",
			"att_xyz789",
		},
		{
			"长 UID",
			"attachment_att_abcdefghij123_my_file_name.docx",
			"att_abcdefghij123",
		},
		{
			"含多个下划线的文件名",
			"attachment_att_id1_file_name_v2_final.pdf",
			"att_id1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := []OpenAIReference{
				{DocumentName: tt.docName, Similarity: 0.5},
			}
			parsed := ParseReferences(refs)

			if len(parsed) != 1 {
				t.Fatalf("数量不正确: %d", len(parsed))
			}
			if parsed[0].Type != ReferenceTypeAttachment {
				t.Errorf("类型不正确: %s", parsed[0].Type)
			}
			if parsed[0].AttachmentUID != tt.expectedUID {
				t.Errorf("AttachmentUID 不正确: 期望 %s, 实际 %s", tt.expectedUID, parsed[0].AttachmentUID)
			}
		})
	}
}

func TestParseReferences_AttachmentWithoutFilename(t *testing.T) {
	// "attachment_att_xyz789" → UID 后无文件名 → 整个 rest 作为 UID
	refs := []OpenAIReference{
		{DocumentName: "attachment_att_xyz789", Similarity: 0.5},
	}
	parsed := ParseReferences(refs)

	if parsed[0].Type != ReferenceTypeAttachment {
		t.Errorf("类型不正确: %s", parsed[0].Type)
	}
	// "att_xyz789" 中 "att_" 后的 "xyz789" 没有 "_"，所以整个 rest 就是 UID
	if parsed[0].AttachmentUID != "att_xyz789" {
		t.Errorf("AttachmentUID 不正确: %s", parsed[0].AttachmentUID)
	}
}

func TestParseReferences_AttachmentNonAttPrefix(t *testing.T) {
	// "attachment_customid_report.pdf" → UID 不以 "att_" 开头
	refs := []OpenAIReference{
		{DocumentName: "attachment_customid_report.pdf", Similarity: 0.5},
	}
	parsed := ParseReferences(refs)

	if parsed[0].Type != ReferenceTypeAttachment {
		t.Errorf("类型不正确: %s", parsed[0].Type)
	}
	if parsed[0].AttachmentUID != "customid" {
		t.Errorf("AttachmentUID 不正确: %s", parsed[0].AttachmentUID)
	}
}

func TestParseReferences_AttachmentEmptyAfterPrefix(t *testing.T) {
	// "attachment_" → 去前缀后为空 → 降级为 unknown
	refs := []OpenAIReference{
		{DocumentName: "attachment_", Similarity: 0.5},
	}
	parsed := ParseReferences(refs)

	if parsed[0].Type != ReferenceTypeUnknown {
		t.Errorf("空附件 UID 应降级为 unknown: %s", parsed[0].Type)
	}
}

// ==================== extractAttachmentUID 单元测试 ====================

func TestExtractAttachmentUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"标准 att_ 前缀", "attachment_att_xyz789_report.pdf", "att_xyz789"},
		{"无文件名", "attachment_att_abc123", "att_abc123"},
		{"非 att_ 前缀", "attachment_customid_file.txt", "customid"},
		{"空", "attachment_", ""},
		{"只有 att_", "attachment_att_", "att_"},
		{"非 att_ 无下划线", "attachment_singleid", "singleid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAttachmentUID(tt.input)
			if result != tt.expected {
				t.Errorf("期望 %q, 实际 %q", tt.expected, result)
			}
		})
	}
}

// ==================== truncateSnippet 测试 ====================

func TestTruncateSnippet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"短英文", "hello", 10, "hello"},
		{"精确长度", "hello", 5, "hello"},
		{"截断英文", "hello world", 5, "hello..."},
		{"空字符串", "", 10, ""},
		{"中文不截断", "你好世界", 10, "你好世界"},
		{"中文截断", "你好世界这是一段很长的中文内容", 4, "你好世界..."},
		{"单个字符", "a", 1, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateSnippet(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("期望 %q, 实际 %q", tt.expected, result)
			}
		})
	}
}

func TestTruncateSnippet_LongContent(t *testing.T) {
	// 生成超过 maxSnippetLen（200）的内容
	longContent := strings.Repeat("这是测试内容", 50) // 300 个 rune
	result := truncateSnippet(longContent, maxSnippetLen)

	runes := []rune(result)
	// 200 个 rune + "..." = 203 个 rune
	if len(runes) != maxSnippetLen+3 {
		t.Errorf("截断后 rune 数量不正确: %d", len(runes))
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("截断后应以 ... 结尾")
	}
}

// ==================== 引用类型常量测试 ====================

func TestReferenceTypeConstants(t *testing.T) {
	if ReferenceTypeMemo != "memo" {
		t.Errorf("ReferenceTypeMemo 不正确: %s", ReferenceTypeMemo)
	}
	if ReferenceTypeAttachment != "attachment" {
		t.Errorf("ReferenceTypeAttachment 不正确: %s", ReferenceTypeAttachment)
	}
	if ReferenceTypeUnknown != "unknown" {
		t.Errorf("ReferenceTypeUnknown 不正确: %s", ReferenceTypeUnknown)
	}
}

// ==================== 完整场景测试 ====================

func TestParseReferences_RealWorldScenario(t *testing.T) {
	// 模拟 RAGFlow 实际返回的引用列表
	refs := []OpenAIReference{
		{
			ID:               "chunk_a1b2c3",
			Content:          "Go 的并发模型基于 CSP（Communicating Sequential Processes），goroutine 是其核心抽象。每个 goroutine 仅占约 2KB 栈空间。",
			DocumentID:       "doc_001",
			DocumentName:     "memo_m_abc123def456.txt",
			DatasetID:        "ds_user42",
			Similarity:       0.92,
			VectorSimilarity: 0.95,
			TermSimilarity:   0.60,
			Positions:        [][]int{{0, 150}},
		},
		{
			ID:               "chunk_d4e5f6",
			Content:          "Channel 是 Go 中 goroutine 之间通信的主要机制",
			DocumentID:       "doc_002",
			DocumentName:     "attachment_att_xyz789_Go并发编程.pdf",
			DatasetID:        "ds_user42",
			Similarity:       0.78,
			VectorSimilarity: 0.80,
			TermSimilarity:   0.45,
		},
		{
			ID:               "chunk_g7h8i9",
			Content:          "不相关的外部文档内容",
			DocumentID:       "doc_003",
			DocumentName:     "external_reference.html",
			DatasetID:        "ds_user42",
			Similarity:       0.35,
			VectorSimilarity: 0.40,
			TermSimilarity:   0.10,
		},
	}

	parsed := ParseReferences(refs)

	if len(parsed) != 3 {
		t.Fatalf("解析结果数量不正确: %d", len(parsed))
	}

	// 第一个: Memo 引用
	if parsed[0].Type != ReferenceTypeMemo {
		t.Errorf("第一个应为 memo: %s", parsed[0].Type)
	}
	if parsed[0].MemoUID != "m_abc123def456" {
		t.Errorf("MemoUID 不正确: %s", parsed[0].MemoUID)
	}
	if parsed[0].Similarity != 0.92 {
		t.Errorf("Similarity 不正确: %f", parsed[0].Similarity)
	}

	// 第二个: Attachment 引用
	if parsed[1].Type != ReferenceTypeAttachment {
		t.Errorf("第二个应为 attachment: %s", parsed[1].Type)
	}
	if parsed[1].AttachmentUID != "att_xyz789" {
		t.Errorf("AttachmentUID 不正确: %s", parsed[1].AttachmentUID)
	}

	// 第三个: 未知类型
	if parsed[2].Type != ReferenceTypeUnknown {
		t.Errorf("第三个应为 unknown: %s", parsed[2].Type)
	}
}
