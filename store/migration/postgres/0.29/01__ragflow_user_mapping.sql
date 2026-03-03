-- ==================== RAGFlow 用户映射表 ====================
-- 记录用户与 RAGFlow Dataset/Assistant 的映射关系

CREATE TABLE IF NOT EXISTS ragflow_user_mapping (
  id SERIAL PRIMARY KEY,
  user_id INT NOT NULL UNIQUE,
  dataset_id VARCHAR(255) NOT NULL,
  dataset_name VARCHAR(255) NOT NULL DEFAULT '',
  assistant_id VARCHAR(255) NOT NULL DEFAULT '',
  document_count INT NOT NULL DEFAULT 0,
  last_sync_ts BIGINT,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ragflow_user_mapping_user_id ON ragflow_user_mapping(user_id);
