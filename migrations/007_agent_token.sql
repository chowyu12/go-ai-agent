ALTER TABLE `agents` ADD COLUMN `token` VARCHAR(64) NOT NULL DEFAULT '' AFTER `timeout`;
ALTER TABLE `agents` ADD UNIQUE KEY `uk_token` (`token`);
