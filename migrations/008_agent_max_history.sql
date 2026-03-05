ALTER TABLE `agents` ADD COLUMN `max_history` INT UNSIGNED NOT NULL DEFAULT 50 COMMENT '会话历史最大条数' AFTER `timeout`;
ALTER TABLE `agents` ADD COLUMN `max_iterations` INT UNSIGNED NOT NULL DEFAULT 10 COMMENT '工具调用最大迭代次数' AFTER `max_history`;
