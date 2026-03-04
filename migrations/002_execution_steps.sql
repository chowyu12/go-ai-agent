-- 执行步骤详情表：记录 Agent 执行过程中每一个步骤
CREATE TABLE IF NOT EXISTS `execution_steps` (
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `message_id`      BIGINT UNSIGNED NOT NULL             COMMENT '关联的 assistant message ID',
    `conversation_id` BIGINT UNSIGNED NOT NULL,
    `step_order`      INT UNSIGNED    NOT NULL DEFAULT 0   COMMENT '步骤序号（从1开始）',
    `step_type`       VARCHAR(32)     NOT NULL DEFAULT ''  COMMENT 'llm_call/tool_call/agent_call',
    `name`            VARCHAR(128)    NOT NULL DEFAULT ''   COMMENT 'tool名称 或 子agent名称 或 模型名称',
    `input`           MEDIUMTEXT      NULL                 COMMENT '步骤输入内容',
    `output`          MEDIUMTEXT      NULL                 COMMENT '步骤输出内容',
    `status`          VARCHAR(16)     NOT NULL DEFAULT 'success' COMMENT 'success/error/pending',
    `error`           TEXT            NULL                 COMMENT '错误信息',
    `duration_ms`     INT UNSIGNED    NOT NULL DEFAULT 0   COMMENT '耗时(毫秒)',
    `tokens_used`     INT UNSIGNED    NOT NULL DEFAULT 0,
    `metadata`        JSON            NULL                 COMMENT '额外元数据(provider, model, temperature等)',
    `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_message_id` (`message_id`),
    KEY `idx_conversation_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 给 messages 表添加 parent_step_id 字段，串联 tool 结果消息与步骤
ALTER TABLE `messages` ADD COLUMN `parent_step_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联的执行步骤ID' AFTER `tokens_used`;
ALTER TABLE `messages` ADD KEY `idx_parent_step_id` (`parent_step_id`);
