-- ==================== ai_message 表字段扩展 ====================
-- 为 P3 OpenAI Compatible API 添加新字段
-- 这些字段存储 RAGFlow 响应的额外元数据

-- reasoning_content: 思考链内容（DeepSeek 风格）
ALTER TABLE ai_message ADD COLUMN IF NOT EXISTS reasoning_content TEXT NOT NULL DEFAULT '';

-- references_json: 引用文档 JSON
-- 格式: [{"memo_uid":"...","type":"memo","content_snippet":"...","similarity":0.85}]
ALTER TABLE ai_message ADD COLUMN IF NOT EXISTS references_json TEXT NOT NULL DEFAULT '';

-- token_usage_json: Token 使用统计（OpenAI 格式）
-- 格式: {"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}
ALTER TABLE ai_message ADD COLUMN IF NOT EXISTS token_usage_json TEXT NOT NULL DEFAULT '';
