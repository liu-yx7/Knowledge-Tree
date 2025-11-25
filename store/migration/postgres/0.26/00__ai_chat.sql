-- ai_conversation table
CREATE TABLE ai_conversation (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  creator_id INTEGER NOT NULL,
  llm_provider TEXT NOT NULL,
  llm_model TEXT NOT NULL,
  system_prompt TEXT NOT NULL DEFAULT '',
  created_ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ai_conversation_creator_id ON ai_conversation(creator_id);
CREATE INDEX idx_ai_conversation_updated_ts ON ai_conversation(updated_ts DESC);

-- ai_message table
CREATE TABLE ai_message (
  id SERIAL PRIMARY KEY,
  conversation_id INTEGER NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL,
  tokens INTEGER NOT NULL DEFAULT 0,
  created_ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (conversation_id) REFERENCES ai_conversation(id) ON DELETE CASCADE
);

CREATE INDEX idx_ai_message_conversation_id ON ai_message(conversation_id);
CREATE INDEX idx_ai_message_created_ts ON ai_message(created_ts);

-- ai_provider_config table
CREATE TABLE ai_provider_config (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  api_key TEXT NOT NULL DEFAULT '',
  api_endpoint TEXT NOT NULL DEFAULT '',
  config TEXT NOT NULL DEFAULT '{}',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ai_provider_config_name ON ai_provider_config(name);
CREATE INDEX idx_ai_provider_config_enabled ON ai_provider_config(enabled);
