-- ==================== RAGFlow 对话表 ====================
-- 存储用户与 RAGFlow Chat Assistant 的对话

CREATE TABLE IF NOT EXISTS ragflow_conversation (
  id SERIAL PRIMARY KEY,
  uid VARCHAR(255) NOT NULL UNIQUE,
  user_id INT NOT NULL,
  ragflow_session_id VARCHAR(255) NOT NULL,
  title VARCHAR(255) NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  row_status VARCHAR(10) NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ragflow_conversation_user_id ON ragflow_conversation(user_id);
CREATE INDEX IF NOT EXISTS idx_ragflow_conversation_session_id ON ragflow_conversation(ragflow_session_id);
