-- system_setting
CREATE TABLE system_setting (
  name TEXT NOT NULL,
  value TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  UNIQUE(name)
);

-- user
CREATE TABLE user (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  username TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL DEFAULT 'USER',
  email TEXT NOT NULL DEFAULT '',
  nickname TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT ''
);

-- user_setting
CREATE TABLE user_setting (
  user_id INTEGER NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  UNIQUE(user_id, key)
);

-- memo
CREATE TABLE memo (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  content TEXT NOT NULL DEFAULT '',
  visibility TEXT NOT NULL CHECK (visibility IN ('PUBLIC', 'PROTECTED', 'PRIVATE')) DEFAULT 'PRIVATE',
  pinned INTEGER NOT NULL CHECK (pinned IN (0, 1)) DEFAULT 0,
  payload TEXT NOT NULL DEFAULT '{}'
);

-- memo_relation
CREATE TABLE memo_relation (
  memo_id INTEGER NOT NULL,
  related_memo_id INTEGER NOT NULL,
  type TEXT NOT NULL,
  UNIQUE(memo_id, related_memo_id, type)
);

-- attachment
CREATE TABLE attachment (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  filename TEXT NOT NULL DEFAULT '',
  blob BLOB DEFAULT NULL,
  type TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  memo_id INTEGER,
  storage_type TEXT NOT NULL DEFAULT '',
  reference TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}'
);

-- activity
CREATE TABLE activity (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  type TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL CHECK (level IN ('INFO', 'WARN', 'ERROR')) DEFAULT 'INFO',
  payload TEXT NOT NULL DEFAULT '{}'
);

-- idp
CREATE TABLE idp (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  identifier_filter TEXT NOT NULL DEFAULT '',
  config TEXT NOT NULL DEFAULT '{}'
);

-- inbox
CREATE TABLE inbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  sender_id INTEGER NOT NULL,
  receiver_id INTEGER NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '{}'
);

-- reaction
CREATE TABLE reaction (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  creator_id INTEGER NOT NULL,
  content_id TEXT NOT NULL,
  reaction_type TEXT NOT NULL,
  UNIQUE(creator_id, content_id, reaction_type)
);

-- ai_conversation: stores AI chat conversations
CREATE TABLE ai_conversation (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  user_id INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT 'New Chat',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  model TEXT NOT NULL DEFAULT '',
  provider TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_ai_conversation_user_id ON ai_conversation(user_id);
CREATE INDEX idx_ai_conversation_created_ts ON ai_conversation(created_ts);

-- ai_message: stores individual messages in AI conversations
CREATE TABLE ai_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  conversation_id INTEGER NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
  content TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  token_count INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY (conversation_id) REFERENCES ai_conversation(id) ON DELETE CASCADE
);

CREATE INDEX idx_ai_message_conversation_id ON ai_message(conversation_id);
CREATE INDEX idx_ai_message_created_ts ON ai_message(created_ts);

-- subscription: stores follow relationships between users
CREATE TABLE subscription (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  follower_id INTEGER NOT NULL,
  following_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  UNIQUE(follower_id, following_id),
  FOREIGN KEY (follower_id) REFERENCES user(id) ON DELETE CASCADE,
  FOREIGN KEY (following_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX idx_subscription_follower ON subscription(follower_id);
CREATE INDEX idx_subscription_following ON subscription(following_id);

-- ragflow_user_mapping: stores user to RAGFlow dataset/assistant mapping
CREATE TABLE ragflow_user_mapping (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL UNIQUE,
  dataset_id TEXT NOT NULL,
  dataset_name TEXT NOT NULL DEFAULT '',
  assistant_id TEXT NOT NULL DEFAULT '',
  document_count INTEGER NOT NULL DEFAULT 0,
  last_sync_ts BIGINT,
  ragflow_user_id TEXT NOT NULL DEFAULT '',
  ragflow_email TEXT NOT NULL DEFAULT '',
  ragflow_password TEXT NOT NULL DEFAULT '',
  api_key TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX idx_ragflow_user_mapping_user_id ON ragflow_user_mapping(user_id);

-- content_sync_state: tracks RAGFlow sync status for memos and attachments
CREATE TABLE content_sync_state (
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

CREATE INDEX idx_content_sync_state_owner_id ON content_sync_state(owner_id);
CREATE INDEX idx_content_sync_state_status ON content_sync_state(ragflow_status);

-- ragflow_conversation: stores RAGFlow chat conversations
CREATE TABLE ragflow_conversation (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  user_id INTEGER NOT NULL,
  ragflow_session_id TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX idx_ragflow_conversation_user_id ON ragflow_conversation(user_id);
CREATE INDEX idx_ragflow_conversation_session_id ON ragflow_conversation(ragflow_session_id);

-- ragflow_message: stores messages in RAGFlow conversations
CREATE TABLE ragflow_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  conversation_id INTEGER NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL DEFAULT '',
  references_json TEXT NOT NULL DEFAULT '[]',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (conversation_id) REFERENCES ragflow_conversation(id) ON DELETE CASCADE
);

CREATE INDEX idx_ragflow_message_conversation_id ON ragflow_message(conversation_id);
CREATE INDEX idx_ragflow_message_created_ts ON ragflow_message(created_ts);
