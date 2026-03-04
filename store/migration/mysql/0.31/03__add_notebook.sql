-- notebook: stores notebook (笔记集) collections, each mapping to an independent RAGFlow Dataset.
-- Each user has exactly one default notebook (is_default=1).
CREATE TABLE `notebook` (
  `id` INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(256) NOT NULL UNIQUE,
  `creator_id` INT NOT NULL,
  `title` VARCHAR(256) NOT NULL DEFAULT '',
  `icon` VARCHAR(64) NOT NULL DEFAULT '',
  `is_default` TINYINT(1) NOT NULL DEFAULT 0,
  `dataset_id` VARCHAR(255) NOT NULL DEFAULT '',
  `row_status` VARCHAR(256) NOT NULL DEFAULT 'NORMAL',
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  FOREIGN KEY (`creator_id`) REFERENCES `user`(`id`) ON DELETE CASCADE,
  INDEX `idx_notebook_creator_id` (`creator_id`)
);

-- Partial unique index via generated column workaround for MySQL (no WHERE in UNIQUE INDEX).
-- Only one default notebook per user is enforced at application level.

-- Create a default notebook for each existing user, reusing their existing RAGFlow dataset.
INSERT INTO `notebook` (`uid`, `creator_id`, `title`, `icon`, `is_default`, `dataset_id`)
SELECT
  UUID() AS `uid`,
  `u`.`id` AS `creator_id`,
  'Default' AS `title`,
  '📚' AS `icon`,
  1 AS `is_default`,
  COALESCE(`r`.`dataset_id`, '') AS `dataset_id`
FROM `user` `u`
LEFT JOIN `ragflow_user_mapping` `r` ON `u`.`id` = `r`.`user_id`;

-- Add notebook_id column to memo table, referencing the notebook.
ALTER TABLE `memo` ADD COLUMN `notebook_id` INT DEFAULT NULL;
ALTER TABLE `memo` ADD CONSTRAINT `fk_memo_notebook` FOREIGN KEY (`notebook_id`) REFERENCES `notebook`(`id`) ON DELETE SET NULL;

-- Backfill: assign all existing memos to their creator's default notebook.
UPDATE `memo` `m`
INNER JOIN `notebook` `nb` ON `nb`.`creator_id` = `m`.`creator_id` AND `nb`.`is_default` = 1
SET `m`.`notebook_id` = `nb`.`id`;
