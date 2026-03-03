-- ==================== P2: RAGFlow 用户认证字段扩展 ====================
-- 为 ragflow_user_mapping 表新增认证相关字段，支持用户无感知 RAGFlow 账户自动配置

ALTER TABLE ragflow_user_mapping ADD COLUMN ragflow_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE ragflow_user_mapping ADD COLUMN ragflow_email TEXT NOT NULL DEFAULT '';
ALTER TABLE ragflow_user_mapping ADD COLUMN ragflow_password TEXT NOT NULL DEFAULT '';
ALTER TABLE ragflow_user_mapping ADD COLUMN api_key TEXT NOT NULL DEFAULT '';
