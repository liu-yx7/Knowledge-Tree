-- ai_conversation: stores chat conversations
-- P3 架构：对话由 Knowtree 本地管理，不绑定 RAGFlow Session
CREATE TABLE IF NOT EXISTS ai_conversation (
  id INT AUTO_INCREMENT PRIMARY KEY,
  uid VARCHAR(256) NOT NULL UNIQUE,
  user_id INT NOT NULL,
  title VARCHAR(512) NOT NULL DEFAULT 'New Chat',
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  row_status VARCHAR(16) NOT NULL DEFAULT 'NORMAL',
  INDEX idx_ai_conversation_user_id (user_id),
  INDEX idx_ai_conversation_created_ts (created_ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ai_message: stores individual messages in conversations
-- P3 架构：支持引用信息、思考链、Token 统计
CREATE TABLE IF NOT EXISTS ai_message (
  id INT AUTO_INCREMENT PRIMARY KEY,
  uid VARCHAR(256) NOT NULL UNIQUE,
  conversation_id INT NOT NULL,
  role VARCHAR(16) NOT NULL,
  content TEXT NOT NULL,
  reasoning_content TEXT NOT NULL,
  references_json TEXT NOT NULL,
  token_usage_json TEXT NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  INDEX idx_ai_message_conversation_id (conversation_id),
  INDEX idx_ai_message_created_ts (created_ts),
  FOREIGN KEY (conversation_id) REFERENCES ai_conversation(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
