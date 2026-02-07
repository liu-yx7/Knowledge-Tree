-- ==================== RAGFlow 消息表 ====================
-- 存储对话中的消息

CREATE TABLE IF NOT EXISTS ragflow_message (
  id INT AUTO_INCREMENT PRIMARY KEY,
  uid VARCHAR(255) NOT NULL UNIQUE,
  conversation_id INT NOT NULL,
  role ENUM('user', 'assistant') NOT NULL,
  content TEXT NOT NULL,
  references_json JSON NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  FOREIGN KEY (conversation_id) REFERENCES ragflow_conversation(id) ON DELETE CASCADE,
  INDEX idx_ragflow_message_conversation_id (conversation_id),
  INDEX idx_ragflow_message_created_ts (created_ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
