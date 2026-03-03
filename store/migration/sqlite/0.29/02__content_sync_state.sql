-- ==================== 内容同步状态表 ====================
-- 记录每个内容项（Memo/Attachment）的 RAGFlow 同步状态

CREATE TABLE IF NOT EXISTS content_sync_state (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  content_type TEXT NOT NULL CHECK (content_type IN ('memo', 'attachment')),
  content_uid TEXT NOT NULL,
  owner_id INTEGER NOT NULL,
  ragflow_status TEXT NOT NULL CHECK (ragflow_status IN ('pending', 'synced', 'failed', 'skipped')) DEFAULT 'pending',
  ragflow_dataset_id TEXT NOT NULL DEFAULT '',
  ragflow_document_id TEXT NOT NULL DEFAULT '',
  ragflow_synced_ts BIGINT,
  ragflow_error TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0,
  next_retry_ts BIGINT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  UNIQUE(content_type, content_uid)
);

CREATE INDEX IF NOT EXISTS idx_content_sync_state_owner_id ON content_sync_state(owner_id);
CREATE INDEX IF NOT EXISTS idx_content_sync_state_status ON content_sync_state(ragflow_status);
CREATE INDEX IF NOT EXISTS idx_content_sync_state_retry ON content_sync_state(next_retry_ts) WHERE ragflow_status IN ('pending', 'failed');
