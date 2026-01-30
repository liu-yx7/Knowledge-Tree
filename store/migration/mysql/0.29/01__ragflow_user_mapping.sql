-- ==================== RAGFlow 用户映射表 ====================
-- 记录用户与 RAGFlow Dataset/Assistant 的映射关系

CREATE TABLE IF NOT EXISTS ragflow_user_mapping (
  id INT AUTO_INCREMENT PRIMARY KEY,
  user_id INT NOT NULL UNIQUE,
  dataset_id VARCHAR(255) NOT NULL,
  dataset_name VARCHAR(255) NOT NULL DEFAULT '',
  assistant_id VARCHAR(255) NOT NULL DEFAULT '',
  document_count INT NOT NULL DEFAULT 0,
  last_sync_ts BIGINT,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE INDEX idx_ragflow_user_mapping_user_id ON ragflow_user_mapping(user_id);
