-- system_setting
CREATE TABLE system_setting (
  name TEXT NOT NULL PRIMARY KEY,
  value TEXT NOT NULL,
  description TEXT NOT NULL
);

-- user
CREATE TABLE "user" (
  id SERIAL PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  row_status TEXT NOT NULL DEFAULT 'NORMAL',
  username TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL DEFAULT 'USER',
  email TEXT NOT NULL DEFAULT '',
  nickname TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  avatar_url TEXT NOT NULL,
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
  id SERIAL PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  row_status TEXT NOT NULL DEFAULT 'NORMAL',
  content TEXT NOT NULL,
  visibility TEXT NOT NULL DEFAULT 'PRIVATE',
  pinned BOOLEAN NOT NULL DEFAULT FALSE,
  payload JSONB NOT NULL DEFAULT '{}'
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
  id SERIAL PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  filename TEXT NOT NULL,
  blob BYTEA,
  type TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  memo_id INTEGER DEFAULT NULL,
  storage_type TEXT NOT NULL DEFAULT '',
  reference TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}'
);

-- activity
CREATE TABLE activity (
  id SERIAL PRIMARY KEY,
  creator_id INTEGER NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  type TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL DEFAULT 'INFO',
  payload JSONB NOT NULL DEFAULT '{}'
);

-- idp
CREATE TABLE idp (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  identifier_filter TEXT NOT NULL DEFAULT '',
  config JSONB NOT NULL DEFAULT '{}'
);

-- inbox
CREATE TABLE inbox (
  id SERIAL PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  sender_id INTEGER NOT NULL,
  receiver_id INTEGER NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL
);

-- reaction
CREATE TABLE reaction (
  id SERIAL PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  creator_id INTEGER NOT NULL,
  content_id TEXT NOT NULL,
  reaction_type TEXT NOT NULL,
  UNIQUE(creator_id, content_id, reaction_type)
);

-- ai_conversation: stores AI chat conversations
CREATE TABLE ai_conversation (
  id SERIAL PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  user_id INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT 'New Chat',
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  row_status TEXT NOT NULL DEFAULT 'NORMAL',
  model TEXT NOT NULL DEFAULT '',
  provider TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_ai_conversation_user_id ON ai_conversation(user_id);
CREATE INDEX idx_ai_conversation_created_ts ON ai_conversation(created_ts);

-- ai_message: stores individual messages in AI conversations
CREATE TABLE ai_message (
  id SERIAL PRIMARY KEY,
  uid TEXT NOT NULL UNIQUE,
  conversation_id INTEGER NOT NULL REFERENCES ai_conversation(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  token_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_ai_message_conversation_id ON ai_message(conversation_id);
CREATE INDEX idx_ai_message_created_ts ON ai_message(created_ts);

-- subscription: stores follow relationships between users
CREATE TABLE subscription (
  id SERIAL PRIMARY KEY,
  follower_id INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  following_id INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  UNIQUE(follower_id, following_id)
);

CREATE INDEX idx_subscription_follower_id ON subscription(follower_id);
CREATE INDEX idx_subscription_following_id ON subscription(following_id);

-- ==================== RAGFlow 用户映射表 ====================
-- 记录用户与 RAGFlow Dataset/Assistant 的映射关系

CREATE TABLE ragflow_user_mapping (
  id SERIAL PRIMARY KEY,
  user_id INT NOT NULL UNIQUE,
  dataset_id VARCHAR(255) NOT NULL,
  dataset_name VARCHAR(255) NOT NULL DEFAULT '',
  assistant_id VARCHAR(255) NOT NULL DEFAULT '',
  document_count INT NOT NULL DEFAULT 0,
  last_sync_ts BIGINT,
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  updated_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE INDEX idx_ragflow_user_mapping_user_id ON ragflow_user_mapping(user_id);

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

CREATE TABLE content_sync_state (
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

CREATE INDEX idx_content_sync_state_owner_id ON content_sync_state(owner_id);
CREATE INDEX idx_content_sync_state_status ON content_sync_state(ragflow_status);
CREATE INDEX idx_content_sync_state_retry ON content_sync_state(next_retry_ts) WHERE ragflow_status IN ('pending', 'failed');

-- ==================== RAGFlow 对话表 ====================
-- 存储用户与 RAGFlow Chat Assistant 的对话

CREATE TABLE ragflow_conversation (
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

CREATE INDEX idx_ragflow_conversation_user_id ON ragflow_conversation(user_id);
CREATE INDEX idx_ragflow_conversation_session_id ON ragflow_conversation(ragflow_session_id);

-- ==================== RAGFlow 消息表 ====================
-- 存储对话中的消息

CREATE TABLE ragflow_message (
  id SERIAL PRIMARY KEY,
  uid VARCHAR(255) NOT NULL UNIQUE,
  conversation_id INT NOT NULL,
  role VARCHAR(10) NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL DEFAULT '',
  references_json JSONB NOT NULL DEFAULT '[]',
  created_ts BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  FOREIGN KEY (conversation_id) REFERENCES ragflow_conversation(id) ON DELETE CASCADE
);

CREATE INDEX idx_ragflow_message_conversation_id ON ragflow_message(conversation_id);
CREATE INDEX idx_ragflow_message_created_ts ON ragflow_message(created_ts);
