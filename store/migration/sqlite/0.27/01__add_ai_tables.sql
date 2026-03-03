-- ai_conversation: stores chat conversations
-- P3 架构：对话由 Knowtree 本地管理，不绑定 RAGFlow Session
-- 模型/提供商由 RAGFlow Chat Assistant 配置管理，不在对话级别选择
CREATE TABLE IF NOT EXISTS ai_conversation (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  user_id INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT 'New Chat',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  row_status TEXT NOT NULL CHECK (row_status IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL'
);

CREATE INDEX IF NOT EXISTS idx_ai_conversation_user_id ON ai_conversation(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_conversation_created_ts ON ai_conversation(created_ts);

-- ai_message: stores individual messages in conversations
-- P3 架构：支持引用信息、思考链、Token 统计
CREATE TABLE IF NOT EXISTS ai_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uid TEXT NOT NULL UNIQUE,
  conversation_id INTEGER NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL DEFAULT '',
  reasoning_content TEXT NOT NULL DEFAULT '',
  references_json TEXT NOT NULL DEFAULT '',
  token_usage_json TEXT NOT NULL DEFAULT '',
  created_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (conversation_id) REFERENCES ai_conversation(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ai_message_conversation_id ON ai_message(conversation_id);
CREATE INDEX IF NOT EXISTS idx_ai_message_created_ts ON ai_message(created_ts);
