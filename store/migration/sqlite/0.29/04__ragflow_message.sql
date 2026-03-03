-- ==================== RAGFlow 消息表 ====================
-- 存储对话中的消息（替代旧的 ai_message）

CREATE TABLE IF NOT EXISTS ragflow_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  conversation_id INTEGER NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL DEFAULT '',
  references_json TEXT NOT NULL DEFAULT '[]',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (conversation_id) REFERENCES ragflow_conversation(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ragflow_message_conversation_id ON ragflow_message(conversation_id);
CREATE INDEX IF NOT EXISTS idx_ragflow_message_created_ts ON ragflow_message(created_ts);
