CREATE TABLE IF NOT EXISTS `providers` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `name`       VARCHAR(128)    NOT NULL DEFAULT '',
    `type`       VARCHAR(32)     NOT NULL DEFAULT '' COMMENT 'openai/qwen/kimi/openrouter/newapi',
    `base_url`   VARCHAR(512)    NOT NULL DEFAULT '',
    `api_key`    VARCHAR(512)    NOT NULL DEFAULT '',
    `models`     JSON            NULL     COMMENT '可用模型列表',
    `enabled`    TINYINT(1)      NOT NULL DEFAULT 1,
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `agents` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`          VARCHAR(36)     NOT NULL DEFAULT '',
    `name`          VARCHAR(128)    NOT NULL DEFAULT '',
    `description`   TEXT            NULL,
    `system_prompt` TEXT            NULL,
    `provider_id`   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `model_name`    VARCHAR(128)    NOT NULL DEFAULT '',
    `temperature`   DECIMAL(3,2)    NOT NULL DEFAULT 0.70,
    `max_tokens`    INT UNSIGNED    NOT NULL DEFAULT 2048,
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `tools` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`           VARCHAR(36)     NOT NULL DEFAULT '',
    `name`           VARCHAR(128)    NOT NULL DEFAULT '',
    `description`    TEXT            NULL,
    `function_def`   JSON            NULL     COMMENT 'OpenAI function calling schema',
    `handler_type`   VARCHAR(32)     NOT NULL DEFAULT 'builtin' COMMENT 'builtin/http/script',
    `handler_config` JSON            NULL     COMMENT 'handler 配置',
    `enabled`        TINYINT(1)      NOT NULL DEFAULT 1,
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `skills` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`        VARCHAR(36)     NOT NULL DEFAULT '',
    `name`        VARCHAR(128)    NOT NULL DEFAULT '',
    `description` TEXT            NULL,
    `instruction` TEXT            NULL     COMMENT '技能指令',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `agent_tools` (
    `agent_id` BIGINT UNSIGNED NOT NULL,
    `tool_id`  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`agent_id`, `tool_id`),
    KEY `idx_tool_id` (`tool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `agent_skills` (
    `agent_id` BIGINT UNSIGNED NOT NULL,
    `skill_id` BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`agent_id`, `skill_id`),
    KEY `idx_skill_id` (`skill_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `skill_tools` (
    `skill_id` BIGINT UNSIGNED NOT NULL,
    `tool_id`  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`skill_id`, `tool_id`),
    KEY `idx_tool_id` (`tool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `agent_children` (
    `parent_id` BIGINT UNSIGNED NOT NULL,
    `child_id`  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`parent_id`, `child_id`),
    KEY `idx_child_id` (`child_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `conversations` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`       VARCHAR(36)     NOT NULL DEFAULT '',
    `agent_id`   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `user_id`    VARCHAR(128)    NOT NULL DEFAULT '',
    `title`      VARCHAR(256)    NOT NULL DEFAULT '',
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    KEY `idx_agent_user` (`agent_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `messages` (
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `conversation_id` BIGINT UNSIGNED NOT NULL,
    `role`            VARCHAR(16)     NOT NULL DEFAULT '' COMMENT 'system/user/assistant/tool',
    `content`         MEDIUMTEXT      NULL,
    `tool_calls`      JSON            NULL,
    `tool_call_id`    VARCHAR(64)     NOT NULL DEFAULT '',
    `tokens_used`     INT UNSIGNED    NOT NULL DEFAULT 0,
    `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_conversation_id` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
