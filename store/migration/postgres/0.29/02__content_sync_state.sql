-- ==================== 内容同步状态表 ====================
-- 记录每个内容项（Memo/Attachment）的 RAGFlow 同步状态

DO $$ BEGIN
  CREATE TYPE content_type_enum AS ENUM ('memo', 'attachment');
EXCEPTION
  WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
  CREATE TYPE ragflow_status_enum AS ENUM ('pending', 'synced', 'failed', 'skipped');
EXCEPTION
  WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS content_sync_state (
  id SERIAL PRIMARY KEY,
  content_type content_type_enum NOT NULL,
  content_uid VARCHAR(255) NOT NULL,
  owner_id INT NOT NULL,
  ragflow_status ragflow_status_enum NOT NULL DEFAULT 'pending',
  ragflow_dataset_id VARCHAR(255) NOT NULL DEFAULT '',
  ragflow_document_id VARCHAR(255) NOT NULL DEFAULT '',
  ragflow_synced_ts BIGINT,
  ragflow_error TEXT NOT NULL DEFAULT '',
  content_hash VARCHAR(64) NOT NULL DEFAULT '',
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_ts BIGINT,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  UNIQUE(content_type, content_uid)
);

CREATE INDEX IF NOT EXISTS idx_content_sync_state_owner_id ON content_sync_state(owner_id);
CREATE INDEX IF NOT EXISTS idx_content_sync_state_status ON content_sync_state(ragflow_status);
CREATE INDEX IF NOT EXISTS idx_content_sync_state_retry ON content_sync_state(next_retry_ts) WHERE ragflow_status IN ('pending', 'failed');
