-- ==================== P2: RAGFlow 用户认证字段扩展 ====================
-- 为 ragflow_user_mapping 表新增认证相关字段，支持用户无感知 RAGFlow 账户自动配置

DO $$ BEGIN
  ALTER TABLE ragflow_user_mapping ADD COLUMN ragflow_user_id VARCHAR(255) NOT NULL DEFAULT '';
EXCEPTION
  WHEN duplicate_column THEN null;
END $$;

DO $$ BEGIN
  ALTER TABLE ragflow_user_mapping ADD COLUMN ragflow_email VARCHAR(255) NOT NULL DEFAULT '';
EXCEPTION
  WHEN duplicate_column THEN null;
END $$;

DO $$ BEGIN
  ALTER TABLE ragflow_user_mapping ADD COLUMN ragflow_password VARCHAR(255) NOT NULL DEFAULT '';
EXCEPTION
  WHEN duplicate_column THEN null;
END $$;

DO $$ BEGIN
  ALTER TABLE ragflow_user_mapping ADD COLUMN api_key VARCHAR(255) NOT NULL DEFAULT '';
EXCEPTION
  WHEN duplicate_column THEN null;
END $$;
