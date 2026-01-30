-- ==================== RAGFlow 用户映射表 ====================
-- 记录用户与 RAGFlow Dataset/Assistant 的映射关系

CREATE TABLE IF NOT EXISTS ragflow_user_mapping (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL UNIQUE,
  dataset_id TEXT NOT NULL,
  dataset_name TEXT NOT NULL DEFAULT '',
  assistant_id TEXT NOT NULL DEFAULT '',
  document_count INTEGER NOT NULL DEFAULT 0,
  last_sync_ts BIGINT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ragflow_user_mapping_user_id ON ragflow_user_mapping(user_id);
