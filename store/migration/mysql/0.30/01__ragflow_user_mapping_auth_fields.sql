-- ==================== P2: RAGFlow 用户认证字段扩展 ====================
-- 为 ragflow_user_mapping 表新增认证相关字段，支持用户无感知 RAGFlow 账户自动配置
-- 使用条件判断确保幂等（重复执行不报错）

SET @dbname = DATABASE();

SELECT COUNT(*) INTO @col_exists FROM information_schema.columns WHERE table_schema = @dbname AND table_name = 'ragflow_user_mapping' AND column_name = 'ragflow_user_id';
SET @sql = IF(@col_exists = 0, 'ALTER TABLE `ragflow_user_mapping` ADD COLUMN `ragflow_user_id` VARCHAR(255) NOT NULL DEFAULT \'\'', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT COUNT(*) INTO @col_exists FROM information_schema.columns WHERE table_schema = @dbname AND table_name = 'ragflow_user_mapping' AND column_name = 'ragflow_email';
SET @sql = IF(@col_exists = 0, 'ALTER TABLE `ragflow_user_mapping` ADD COLUMN `ragflow_email` VARCHAR(255) NOT NULL DEFAULT \'\'', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT COUNT(*) INTO @col_exists FROM information_schema.columns WHERE table_schema = @dbname AND table_name = 'ragflow_user_mapping' AND column_name = 'ragflow_password';
SET @sql = IF(@col_exists = 0, 'ALTER TABLE `ragflow_user_mapping` ADD COLUMN `ragflow_password` VARCHAR(255) NOT NULL DEFAULT \'\'', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT COUNT(*) INTO @col_exists FROM information_schema.columns WHERE table_schema = @dbname AND table_name = 'ragflow_user_mapping' AND column_name = 'api_key';
SET @sql = IF(@col_exists = 0, 'ALTER TABLE `ragflow_user_mapping` ADD COLUMN `api_key` VARCHAR(255) NOT NULL DEFAULT \'\'', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
