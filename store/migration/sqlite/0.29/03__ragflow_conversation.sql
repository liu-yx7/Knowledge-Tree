-- ==================== RAGFlow 对话表 ====================
-- 存储用户与 RAGFlow Chat Assistant 的对话（替代旧的 ai_conversation）

CREATE TABLE IF NOT EXISTS ragflow_conversation (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  user_id INTEGER NOT NULL,
  ragflow_session_id TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ragflow_conversation_user_id ON ragflow_conversation(user_id);
CREATE INDEX IF NOT EXISTS idx_ragflow_conversation_session_id ON ragflow_conversation(ragflow_session_id);
