-- system_setting
CREATE TABLE `system_setting` (
  `name` VARCHAR(256) NOT NULL PRIMARY KEY,
  `value` LONGTEXT NOT NULL,
  `description` TEXT NOT NULL
);

-- user
CREATE TABLE `user` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `row_status` VARCHAR(256) NOT NULL DEFAULT 'NORMAL',
  `username` VARCHAR(256) NOT NULL UNIQUE,
  `role` VARCHAR(256) NOT NULL DEFAULT 'USER',
  `email` VARCHAR(256) NOT NULL DEFAULT '',
  `nickname` VARCHAR(256) NOT NULL DEFAULT '',
  `password_hash` VARCHAR(256) NOT NULL,
  `avatar_url` LONGTEXT NOT NULL,
  `description` VARCHAR(256) NOT NULL DEFAULT ''
);

-- user_setting
CREATE TABLE `user_setting` (
  `user_id` INT NOT NULL,
  `key` VARCHAR(256) NOT NULL,
  `value` LONGTEXT NOT NULL,
  UNIQUE(`user_id`,`key`)
);

-- memo
CREATE TABLE `memo` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `row_status` VARCHAR(256) NOT NULL DEFAULT 'NORMAL',
  `content` TEXT NOT NULL,
  `visibility` VARCHAR(256) NOT NULL DEFAULT 'PRIVATE',
  `pinned` BOOLEAN NOT NULL DEFAULT FALSE,
  `payload` JSON NOT NULL
);

-- memo_relation
CREATE TABLE `memo_relation` (
  `memo_id` INT NOT NULL,
  `related_memo_id` INT NOT NULL,
  `type` VARCHAR(256) NOT NULL,
  UNIQUE(`memo_id`,`related_memo_id`,`type`)
);

-- attachment
CREATE TABLE `attachment` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `filename` TEXT NOT NULL,
  `blob` MEDIUMBLOB,
  `type` VARCHAR(256) NOT NULL DEFAULT '',
  `size` INT NOT NULL DEFAULT '0',
  `memo_id` INT DEFAULT NULL,
  `storage_type` VARCHAR(256) NOT NULL DEFAULT '',
  `reference` TEXT NOT NULL DEFAULT (''),
  `payload` TEXT NOT NULL
);

-- activity
CREATE TABLE `activity` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `creator_id` INT NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `type` VARCHAR(256) NOT NULL DEFAULT '',
  `level` VARCHAR(256) NOT NULL DEFAULT 'INFO',
  `payload` TEXT NOT NULL
);

-- idp
CREATE TABLE `idp` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `name` TEXT NOT NULL,
  `type` TEXT NOT NULL,
  `identifier_filter` VARCHAR(256) NOT NULL DEFAULT '',
  `config` TEXT NOT NULL
);

-- inbox
CREATE TABLE `inbox` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `sender_id` INT NOT NULL,
  `receiver_id` INT NOT NULL,
  `status` TEXT NOT NULL,
  `message` TEXT NOT NULL
);

-- reaction
CREATE TABLE `reaction` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `creator_id` INT NOT NULL,
  `content_id` VARCHAR(256) NOT NULL,
  `reaction_type` VARCHAR(256) NOT NULL,
  UNIQUE(`creator_id`,`content_id`,`reaction_type`)  
);

-- ai_conversation: stores AI chat conversations
-- P3 架构：对话由 Knowtree 本地管理，不绑定 RAGFlow Session
CREATE TABLE `ai_conversation` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `user_id` INT NOT NULL,
  `title` VARCHAR(512) NOT NULL DEFAULT 'New Chat',
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `row_status` VARCHAR(16) NOT NULL DEFAULT 'NORMAL',
  INDEX `idx_ai_conversation_user_id` (`user_id`),
  INDEX `idx_ai_conversation_created_ts` (`created_ts`)
);

-- ai_message: stores individual messages in AI conversations
-- P3 架构：支持引用信息、思考链、Token 统计
CREATE TABLE `ai_message` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `conversation_id` INT NOT NULL,
  `role` VARCHAR(16) NOT NULL,
  `content` TEXT NOT NULL,
  `reasoning_content` TEXT NOT NULL,
  `references_json` TEXT NOT NULL,
  `token_usage_json` TEXT NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX `idx_ai_message_conversation_id` (`conversation_id`),
  INDEX `idx_ai_message_created_ts` (`created_ts`),
  FOREIGN KEY (`conversation_id`) REFERENCES `ai_conversation`(`id`) ON DELETE CASCADE
);

-- subscription: stores follow relationships between users
CREATE TABLE `subscription` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `follower_id` INT NOT NULL,
  `following_id` INT NOT NULL,
  `created_ts` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(`follower_id`, `following_id`),
  FOREIGN KEY (`follower_id`) REFERENCES `user`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (`following_id`) REFERENCES `user`(`id`) ON DELETE CASCADE,
  INDEX `idx_subscription_follower_id` (`follower_id`),
  INDEX `idx_subscription_following_id` (`following_id`)
);

-- ==================== RAGFlow 用户映射表 ====================
-- 记录用户与 RAGFlow Dataset/Assistant 的映射关系

CREATE TABLE `ragflow_user_mapping` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `user_id` INT NOT NULL UNIQUE,
  `dataset_id` VARCHAR(255) NOT NULL,
  `dataset_name` VARCHAR(255) NOT NULL DEFAULT '',
  `assistant_id` VARCHAR(255) NOT NULL DEFAULT '',
  `document_count` INT NOT NULL DEFAULT 0,
  `last_sync_ts` BIGINT,
  `ragflow_user_id` VARCHAR(255) NOT NULL DEFAULT '',
  `ragflow_email` VARCHAR(255) NOT NULL DEFAULT '',
  `ragflow_password` VARCHAR(255) NOT NULL DEFAULT '',
  `api_key` VARCHAR(255) NOT NULL DEFAULT '',
  `llm_configured` TINYINT(1) NOT NULL DEFAULT 0,
  `preferred_llm_id` VARCHAR(255) NOT NULL DEFAULT '',
  `dataset_ids` TEXT NOT NULL,
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  FOREIGN KEY (`user_id`) REFERENCES `user`(`id`) ON DELETE CASCADE,
  INDEX `idx_ragflow_user_mapping_user_id` (`user_id`)
);

-- ==================== 内容同步状态表 ====================
-- 记录每个内容项（Memo/Attachment）的 RAGFlow 同步状态

CREATE TABLE `content_sync_state` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `content_type` ENUM('memo', 'attachment') NOT NULL,
  `content_uid` VARCHAR(255) NOT NULL,
  `owner_id` INT NOT NULL,
  `ragflow_status` ENUM('pending', 'synced', 'failed', 'skipped') NOT NULL DEFAULT 'pending',
  `ragflow_dataset_id` VARCHAR(255) NOT NULL DEFAULT '',
  `ragflow_document_id` VARCHAR(255) NOT NULL DEFAULT '',
  `ragflow_synced_ts` BIGINT,
  `ragflow_error` TEXT NOT NULL,
  `content_hash` VARCHAR(64) NOT NULL DEFAULT '',
  `retry_count` INT NOT NULL DEFAULT 0,
  `next_retry_ts` BIGINT,
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  UNIQUE(`content_type`, `content_uid`),
  INDEX `idx_content_sync_state_owner_id` (`owner_id`),
  INDEX `idx_content_sync_state_status` (`ragflow_status`)
);
