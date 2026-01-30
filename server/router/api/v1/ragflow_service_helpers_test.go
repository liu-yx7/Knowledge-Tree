// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/server/router/api/v1/ragflow_service_helpers_test.go
package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== extractMemoUIDFromDocumentName 测试 ====================

func TestExtractMemoUIDFromDocumentName(t *testing.T) {
	tests := []struct {
		name     string
		docName  string
		expected string
	}{
		{
			name:     "有效的 Memo 文档名",
			docName:  "memo_abc123.md",
			expected: "abc123",
		},
		{
			name:     "带有复杂 UID 的文档名",
			docName:  "memo_550e8400-e29b-41d4-a716-446655440000.md",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "无前缀的文档名",
			docName:  "abc123.md",
			expected: "",
		},
		{
			name:     "错误前缀的文档名",
			docName:  "attachment_abc123.md",
			expected: "",
		},
		{
			name:     "空字符串",
			docName:  "",
			expected: "",
		},
		{
			name:     "只有前缀没有 UID",
			docName:  "memo_.md",
			expected: "",
		},
		{
			name:     "没有 .md 后缀",
			docName:  "memo_abc123",
			expected: "abc123",
		},
		{
			name:     "多个 .md 后缀",
			docName:  "memo_test.md.md",
			expected: "test.md",
		},
		{
			name:     "带下划线的 UID",
			docName:  "memo_test_uid_123.md",
			expected: "test_uid_123",
		},
		{
			name:     "memo 作为 UID 的一部分",
			docName:  "memo_memo_inside.md",
			expected: "memo_inside",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMemoUIDFromDocumentName(tt.docName)
			assert.Equal(t, tt.expected, result, "文档名: %s", tt.docName)
		})
	}
}

// ==================== extractMemoTitle 测试 ====================

func TestExtractMemoTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "普通单行内容",
			content:  "这是一个标题",
			expected: "这是一个标题",
		},
		{
			name:     "Markdown 一级标题",
			content:  "# 这是标题\n\n正文内容",
			expected: "这是标题",
		},
		{
			name:     "Markdown 二级标题",
			content:  "## 二级标题\n\n正文内容",
			expected: "二级标题", // TrimLeft 会移除所有 # 和空格
		},
		{
			name:     "多行内容取第一行",
			content:  "第一行内容\n第二行内容\n第三行内容",
			expected: "第一行内容",
		},
		{
			name:     "空字符串",
			content:  "",
			expected: "",
		},
		{
			name:     "只有空格和换行",
			content:  "   \n   ",
			expected: "   \n   ", // 空格被 SplitN 保留，然后整个内容作为 fallback
		},
		{
			name:     "第一行为空，后面有内容",
			content:  "\n实际内容从第二行开始",
			expected: "\n实际内容从第二行开始", // 第一行为空，触发 fallback 逻辑
		},
		{
			name:     "刚好 50 字符",
			content:  "12345678901234567890123456789012345678901234567890",
			expected: "12345678901234567890123456789012345678901234567890",
		},
		{
			name:     "带有前后空格的内容",
			content:  "  标题内容带空格  \n正文",
			expected: "标题内容带空格",
		},
		{
			name:     "多个 # 号会被完全移除",
			content:  "### 三级标题",
			expected: "三级标题", // TrimLeft 移除所有 # 和空格
		},
		{
			name:     "Unicode 内容",
			content:  "🎉 庆祝标题 🎊\n内容",
			expected: "🎉 庆祝标题 🎊",
		},
		{
			name:     "代码块开头",
			content:  "```go\nfunc main() {}\n```",
			expected: "```go",
		},
		{
			name:     "引用块开头",
			content:  "> 这是引用内容\n> 第二行引用",
			expected: "> 这是引用内容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMemoTitle(tt.content)
			assert.Equal(t, tt.expected, result, "内容: %q", tt.content)
		})
	}
}

// ==================== 边界条件测试 ====================

func TestExtractMemoTitle_LongContent(t *testing.T) {
	// 测试非常长的内容（第一行超过 50 字符）
	longLine := ""
	for i := 0; i < 100; i++ {
		longLine += "a"
	}
	content := longLine + "\n第二行"

	result := extractMemoTitle(content)
	assert.True(t, len(result) <= 53, "标题长度不应超过 53（50 + '...'）")
	assert.Contains(t, result, "...", "超长标题应包含省略号")
}

func TestExtractMemoTitle_LongUnicodeContent(t *testing.T) {
	// 测试超过 50 字节但可能不超过 50 字符的 Unicode 内容
	// 注意：Go 的 len() 返回字节数，不是字符数
	content := "这是一个非常非常非常非常非常非常非常非常非常非常非常非常长的标题内容"
	result := extractMemoTitle(content)
	// 函数按字节截断，可能会截断 UTF-8 字符
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "...", "超长内容应包含省略号")
}

func TestExtractMemoTitle_EmptyFirstLine(t *testing.T) {
	// 第一行为空，但内容不为空
	content := "\n\n\n实际内容"
	result := extractMemoTitle(content)
	// 因为 SplitN 只分成 2 部分，第一部分是空字符串
	// 然后会取 content 的前 50 字符作为 fallback
	assert.NotEmpty(t, result)
}

func TestExtractMemoUIDFromDocumentName_EdgeCases(t *testing.T) {
	// 测试只有 "memo_" 前缀
	result := extractMemoUIDFromDocumentName("memo_")
	assert.Empty(t, result)

	// 测试 "memo" 不带下划线
	result = extractMemoUIDFromDocumentName("memo")
	assert.Empty(t, result)

	// 测试大小写敏感
	result = extractMemoUIDFromDocumentName("MEMO_abc.md")
	assert.Empty(t, result, "应该区分大小写")

	result = extractMemoUIDFromDocumentName("Memo_abc.md")
	assert.Empty(t, result, "应该区分大小写")
}
