-- notebook: stores notebook (笔记集) collections, each mapping to an independent RAGFlow Dataset.
-- Each user has exactly one default notebook (is_default=1).
CREATE TABLE `notebook` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `uid` TEXT NOT NULL UNIQUE,
  `creator_id` INTEGER NOT NULL,
  `title` TEXT NOT NULL DEFAULT '',
  `icon` TEXT NOT NULL DEFAULT '',
  `is_default` INTEGER NOT NULL CHECK (`is_default` IN (0, 1)) DEFAULT 0,
  `dataset_id` TEXT NOT NULL DEFAULT '',
  `row_status` TEXT NOT NULL CHECK (`row_status` IN ('NORMAL', 'ARCHIVED')) DEFAULT 'NORMAL',
  `created_ts` BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  `updated_ts` BIGINT NOT NULL DEFAULT (strftime('%s', 'now')),
  FOREIGN KEY (`creator_id`) REFERENCES `user`(`id`) ON DELETE CASCADE
);

CREATE INDEX `idx_notebook_creator_id` ON `notebook`(`creator_id`);
CREATE UNIQUE INDEX `idx_notebook_default` ON `notebook`(`creator_id`) WHERE `is_default` = 1;

-- Create a default notebook for each existing user, reusing their existing RAGFlow dataset.
INSERT INTO `notebook` (`uid`, `creator_id`, `title`, `icon`, `is_default`, `dataset_id`)
SELECT
  lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))) AS `uid`,
  `u`.`id` AS `creator_id`,
  'Default' AS `title`,
  '📚' AS `icon`,
  1 AS `is_default`,
  COALESCE(`r`.`dataset_id`, '') AS `dataset_id`
FROM `user` `u`
LEFT JOIN `ragflow_user_mapping` `r` ON `u`.`id` = `r`.`user_id`;

-- Add notebook_id column to memo table, referencing the notebook.
ALTER TABLE `memo` ADD COLUMN `notebook_id` INTEGER REFERENCES `notebook`(`id`) ON DELETE SET NULL;

-- Backfill: assign all existing memos to their creator's default notebook.
UPDATE `memo` SET `notebook_id` = (
  SELECT `nb`.`id` FROM `notebook` `nb`
  WHERE `nb`.`creator_id` = `memo`.`creator_id` AND `nb`.`is_default` = 1
);
