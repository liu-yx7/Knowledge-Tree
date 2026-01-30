-- ==================== RAGFlow 对话表 ====================
-- 存储用户与 RAGFlow Chat Assistant 的对话

CREATE TABLE IF NOT EXISTS ragflow_conversation (
  id INT AUTO_INCREMENT PRIMARY KEY,
  uid VARCHAR(255) NOT NULL UNIQUE,
  user_id INT NOT NULL,
  ragflow_session_id VARCHAR(255) NOT NULL,
  title VARCHAR(255) NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  row_status ENUM('NORMAL', 'ARCHIVED') NOT NULL DEFAULT 'NORMAL',
  FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE INDEX idx_ragflow_conversation_user_id ON ragflow_conversation(user_id);
CREATE INDEX idx_ragflow_conversation_session_id ON ragflow_conversation(ragflow_session_id);
