-- notebook: stores notebook (笔记集) collections, each mapping to an independent RAGFlow Dataset.
-- Each user has exactly one default notebook (is_default=true).
CREATE TABLE notebook (
  id SERIAL PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  icon TEXT NOT NULL DEFAULT '',
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  dataset_id VARCHAR(255) NOT NULL DEFAULT '',
  row_status TEXT NOT NULL DEFAULT 'NORMAL',
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  FOREIGN KEY (creator_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE INDEX idx_notebook_creator_id ON notebook(creator_id);
CREATE UNIQUE INDEX idx_notebook_default ON notebook(creator_id) WHERE is_default = TRUE;

-- Create a default notebook for each existing user, reusing their existing RAGFlow dataset.
INSERT INTO notebook (uid, creator_id, title, icon, is_default, dataset_id)
SELECT
  gen_random_uuid()::TEXT AS uid,
  u.id AS creator_id,
  'Default' AS title,
  '📚' AS icon,
  TRUE AS is_default,
  COALESCE(r.dataset_id, '') AS dataset_id
FROM "user" u
LEFT JOIN ragflow_user_mapping r ON u.id = r.user_id;

-- Add notebook_id column to memo table, referencing the notebook.
ALTER TABLE memo ADD COLUMN notebook_id INTEGER REFERENCES notebook(id) ON DELETE SET NULL;

-- Backfill: assign all existing memos to their creator's default notebook.
UPDATE memo SET notebook_id = nb.id
FROM notebook nb
WHERE nb.creator_id = memo.creator_id AND nb.is_default = TRUE;
