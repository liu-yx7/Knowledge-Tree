-- ==================== RAGFlow 消息表 ====================
-- 存储对话中的消息

CREATE TABLE IF NOT EXISTS ragflow_message (
  id SERIAL PRIMARY KEY,
  uid VARCHAR(255) NOT NULL UNIQUE,
  conversation_id INT NOT NULL,
  role VARCHAR(10) NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL DEFAULT '',
  references_json JSONB NOT NULL DEFAULT '[]',
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  FOREIGN KEY (conversation_id) REFERENCES ragflow_conversation(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ragflow_message_conversation_id ON ragflow_message(conversation_id);
CREATE INDEX IF NOT EXISTS idx_ragflow_message_created_ts ON ragflow_message(created_ts);
