package ragflow_test

import (
	"testing"

	"github.com/usememos/memos/plugin/ragflow"
)

// ==================== ParseReferences 测试 ====================

func TestParseReferences_MemoType(t *testing.T) {
	refs := []ragflow.OpenAIReference{
		{
			ID:           "chunk_1",
			Content:      "Go 的 goroutine 是轻量级线程",
			DocumentID:   "doc_1",
			DocumentName: "memo_m_abc123def456.txt",
			DatasetID:    "ds_1",
			Similarity:   0.85,
		},
	}

	parsed := ragflow.ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeMemo {
		t.Errorf("类型应为 memo, 实际 = %s", parsed[0].Type)
	}
	if parsed[0].MemoUID != "m_abc123def456" {
		t.Errorf("MemoUID 不匹配: %s", parsed[0].MemoUID)
	}
	if parsed[0].AttachmentUID != "" {
		t.Errorf("AttachmentUID 应为空: %s", parsed[0].AttachmentUID)
	}
	if parsed[0].Similarity != 0.85 {
		t.Errorf("Similarity 不匹配: %f", parsed[0].Similarity)
	}
	if parsed[0].DocumentName != "memo_m_abc123def456.txt" {
		t.Errorf("DocumentName 不匹配: %s", parsed[0].DocumentName)
	}
	if parsed[0].ContentSnippet != "Go 的 goroutine 是轻量级线程" {
		t.Errorf("ContentSnippet 不匹配: %s", parsed[0].ContentSnippet)
	}
}

func TestParseReferences_AttachmentType(t *testing.T) {
	refs := []ragflow.OpenAIReference{
		{
			ID:           "chunk_2",
			Content:      "CSP 模型与 channel 通信",
			DocumentID:   "doc_2",
			DocumentName: "attachment_att_xyz789_report.pdf",
			DatasetID:    "ds_1",
			Similarity:   0.72,
		},
	}

	parsed := ragflow.ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeAttachment {
		t.Errorf("类型应为 attachment, 实际 = %s", parsed[0].Type)
	}
	if parsed[0].AttachmentUID != "att_xyz789" {
		t.Errorf("AttachmentUID 不匹配: %s", parsed[0].AttachmentUID)
	}
	if parsed[0].MemoUID != "" {
		t.Errorf("MemoUID 应为空: %s", parsed[0].MemoUID)
	}
}

func TestParseReferences_UnknownType(t *testing.T) {
	refs := []ragflow.OpenAIReference{
		{
			ID:           "chunk_3",
			Content:      "一些内容",
			DocumentName: "random_file.docx",
			Similarity:   0.60,
		},
	}

	parsed := ragflow.ParseReferences(refs)

	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeUnknown {
		t.Errorf("类型应为 unknown, 实际 = %s", parsed[0].Type)
	}
	if parsed[0].MemoUID != "" || parsed[0].AttachmentUID != "" {
		t.Error("unknown 类型的 UID 应为空")
	}
}

func TestParseReferences_MixedTypes(t *testing.T) {
	refs := []ragflow.OpenAIReference{
		{DocumentName: "memo_m_001.txt", Content: "内容1", Similarity: 0.9},
		{DocumentName: "attachment_att_002_file.pdf", Content: "内容2", Similarity: 0.8},
		{DocumentName: "unknown_doc.txt", Content: "内容3", Similarity: 0.5},
	}

	parsed := ragflow.ParseReferences(refs)

	if len(parsed) != 3 {
		t.Fatalf("期望 3 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeMemo {
		t.Errorf("第 1 个应为 memo, 实际 = %s", parsed[0].Type)
	}
	if parsed[1].Type != ragflow.ReferenceTypeAttachment {
		t.Errorf("第 2 个应为 attachment, 实际 = %s", parsed[1].Type)
	}
	if parsed[2].Type != ragflow.ReferenceTypeUnknown {
		t.Errorf("第 3 个应为 unknown, 实际 = %s", parsed[2].Type)
	}
}

func TestParseReferences_EmptyInput(t *testing.T) {
	parsed := ragflow.ParseReferences(nil)
	if parsed != nil {
		t.Errorf("nil 输入应返回 nil, 实际 = %v", parsed)
	}

	parsed = ragflow.ParseReferences([]ragflow.OpenAIReference{})
	if parsed != nil {
		t.Errorf("空切片输入应返回 nil, 实际 = %v", parsed)
	}
}

// ==================== 边界情况测试 ====================

func TestParseReferences_MemoWithoutTxtSuffix(t *testing.T) {
	// memo_ 前缀但无 .txt 后缀（异常格式，仍应解析）
	refs := []ragflow.OpenAIReference{
		{DocumentName: "memo_m_abc123", Similarity: 0.7},
	}

	parsed := ragflow.ParseReferences(refs)
	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeMemo {
		t.Errorf("类型应为 memo, 实际 = %s", parsed[0].Type)
	}
	if parsed[0].MemoUID != "m_abc123" {
		t.Errorf("MemoUID 不匹配: %s", parsed[0].MemoUID)
	}
}

func TestParseReferences_AttachmentWithComplexFilename(t *testing.T) {
	// 附件文件名中含多个下划线
	refs := []ragflow.OpenAIReference{
		{DocumentName: "attachment_att_xyz789_my_complex_report.pdf", Similarity: 0.6},
	}

	parsed := ragflow.ParseReferences(refs)
	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeAttachment {
		t.Errorf("类型应为 attachment, 实际 = %s", parsed[0].Type)
	}
	if parsed[0].AttachmentUID != "att_xyz789" {
		t.Errorf("AttachmentUID 不匹配: %s (期望 att_xyz789)", parsed[0].AttachmentUID)
	}
}

func TestParseReferences_AttachmentWithoutFilename(t *testing.T) {
	// 附件无原始文件名（边缘情况）
	refs := []ragflow.OpenAIReference{
		{DocumentName: "attachment_att_xyz789", Similarity: 0.5},
	}

	parsed := ragflow.ParseReferences(refs)
	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeAttachment {
		t.Errorf("类型应为 attachment, 实际 = %s", parsed[0].Type)
	}
	if parsed[0].AttachmentUID != "att_xyz789" {
		t.Errorf("AttachmentUID 不匹配: %s", parsed[0].AttachmentUID)
	}
}

func TestParseReferences_EmptyDocumentName(t *testing.T) {
	refs := []ragflow.OpenAIReference{
		{DocumentName: "", Similarity: 0.3},
	}

	parsed := ragflow.ParseReferences(refs)
	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}
	if parsed[0].Type != ragflow.ReferenceTypeUnknown {
		t.Errorf("空 document_name 应为 unknown, 实际 = %s", parsed[0].Type)
	}
}

func TestParseReferences_ContentSnippetTruncation(t *testing.T) {
	// 生成超过 200 字符的内容
	longContent := ""
	for i := 0; i < 250; i++ {
		longContent += "字"
	}

	refs := []ragflow.OpenAIReference{
		{DocumentName: "memo_m_001.txt", Content: longContent, Similarity: 0.8},
	}

	parsed := ragflow.ParseReferences(refs)
	if len(parsed) != 1 {
		t.Fatalf("期望 1 个引用, 实际 = %d", len(parsed))
	}

	// 截取 200 个 rune + "..."
	expectedLen := 200*3 + 3 // 200 个中文字符（每个 3 字节）+ "..."
	snippetRunes := []rune(parsed[0].ContentSnippet)
	if len(snippetRunes) != 203 { // 200 runes + 3 chars for "..."
		t.Errorf("ContentSnippet 截取长度不正确: %d runes", len(snippetRunes))
	}
	_ = expectedLen
}

func TestParseReferences_ShortContentNoTruncation(t *testing.T) {
	refs := []ragflow.OpenAIReference{
		{DocumentName: "memo_m_001.txt", Content: "短内容", Similarity: 0.8},
	}

	parsed := ragflow.ParseReferences(refs)
	if parsed[0].ContentSnippet != "短内容" {
		t.Errorf("短内容不应被截取: %s", parsed[0].ContentSnippet)
	}
}

// ==================== Memo UID 格式变体测试 ====================

func TestParseReferences_MemoUIDVariants(t *testing.T) {
	tests := []struct {
		name         string
		documentName string
		expectedUID  string
		expectedType string
	}{
		{
			name:         "标准格式",
			documentName: "memo_m_abc123.txt",
			expectedUID:  "m_abc123",
			expectedType: ragflow.ReferenceTypeMemo,
		},
		{
			name:         "长 UID",
			documentName: "memo_m_abc123def456ghi789.txt",
			expectedUID:  "m_abc123def456ghi789",
			expectedType: ragflow.ReferenceTypeMemo,
		},
		{
			name:         "仅 memo_ 前缀（无内容）",
			documentName: "memo_.txt",
			expectedUID:  "",
			expectedType: ragflow.ReferenceTypeUnknown,
		},
		{
			name:         "memo_ 前缀但内容只有 .txt",
			documentName: "memo_.txt",
			expectedUID:  "",
			expectedType: ragflow.ReferenceTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := []ragflow.OpenAIReference{
				{DocumentName: tt.documentName, Similarity: 0.5},
			}
			parsed := ragflow.ParseReferences(refs)
			if len(parsed) != 1 {
				t.Fatalf("期望 1 个引用")
			}
			if parsed[0].Type != tt.expectedType {
				t.Errorf("类型不匹配: 期望 %s, 实际 %s", tt.expectedType, parsed[0].Type)
			}
			if tt.expectedType == ragflow.ReferenceTypeMemo && parsed[0].MemoUID != tt.expectedUID {
				t.Errorf("MemoUID 不匹配: 期望 %q, 实际 %q", tt.expectedUID, parsed[0].MemoUID)
			}
		})
	}
}
