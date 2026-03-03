-- P4: 添加 LLM 配置相关字段到 ragflow_user_mapping 表
-- llm_configured: 标记用户是否已配置 LLM 提供商（百炼）
-- preferred_llm_id: 用户偏好的 LLM 模型 ID
-- dataset_ids: 用户选择的 Dataset ID 列表（JSON 数组字符串）

ALTER TABLE ragflow_user_mapping ADD COLUMN llm_configured BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE ragflow_user_mapping ADD COLUMN preferred_llm_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE ragflow_user_mapping ADD COLUMN dataset_ids TEXT NOT NULL DEFAULT '[]';
