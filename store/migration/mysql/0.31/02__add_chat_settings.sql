-- P4 Phase 2: 添加 quote_enabled 和 reasoning_enabled 字段
-- 用于控制 AI 对话的 RAG 引用和深度研究功能

ALTER TABLE `ragflow_user_mapping` ADD COLUMN `quote_enabled` BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE `ragflow_user_mapping` ADD COLUMN `reasoning_enabled` BOOLEAN NOT NULL DEFAULT FALSE;
