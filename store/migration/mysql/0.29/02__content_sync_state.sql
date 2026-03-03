-- ==================== 内容同步状态表 ====================
-- 记录每个内容项（Memo/Attachment）的 RAGFlow 同步状态

CREATE TABLE IF NOT EXISTS content_sync_state (
  id INT AUTO_INCREMENT PRIMARY KEY,
  content_type ENUM('memo', 'attachment') NOT NULL,
  content_uid VARCHAR(255) NOT NULL,
  owner_id INT NOT NULL,
  ragflow_status ENUM('pending', 'synced', 'failed', 'skipped') NOT NULL DEFAULT 'pending',
  ragflow_dataset_id VARCHAR(255) NOT NULL DEFAULT '',
  ragflow_document_id VARCHAR(255) NOT NULL DEFAULT '',
  ragflow_synced_ts BIGINT,
  ragflow_error TEXT NOT NULL,
  content_hash VARCHAR(64) NOT NULL DEFAULT '',
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_ts BIGINT,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  UNIQUE KEY uk_content_type_uid (content_type, content_uid),
  INDEX idx_content_sync_state_owner_id (owner_id),
  INDEX idx_content_sync_state_status (ragflow_status),
  INDEX idx_content_sync_state_retry (next_retry_ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
