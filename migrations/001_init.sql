-- ============================================================
--  Go AI Agent - 数据库初始化脚本
-- ============================================================

-- 模型供应商
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
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- Agent
CREATE TABLE IF NOT EXISTS `agents` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`           VARCHAR(36)     NOT NULL DEFAULT '',
    `name`           VARCHAR(128)    NOT NULL DEFAULT '',
    `description`    TEXT            NULL,
    `system_prompt`  TEXT            NULL,
    `provider_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `model_name`     VARCHAR(128)    NOT NULL DEFAULT '',
    `temperature`    DECIMAL(3, 2)   NOT NULL DEFAULT 0.70,
    `max_tokens`     INT UNSIGNED    NOT NULL DEFAULT 4096,
    `timeout`        INT UNSIGNED    NOT NULL DEFAULT 120   COMMENT '执行超时(秒)',
    `max_history`    INT UNSIGNED    NOT NULL DEFAULT 50    COMMENT '会话历史最大条数',
    `max_iterations` INT UNSIGNED    NOT NULL DEFAULT 10    COMMENT '工具调用最大迭代次数',
    `token`          VARCHAR(64)     NOT NULL DEFAULT '',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    UNIQUE KEY `uk_token` (`token`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 工具
CREATE TABLE IF NOT EXISTS `tools` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`           VARCHAR(36)     NOT NULL DEFAULT '',
    `name`           VARCHAR(128)    NOT NULL DEFAULT '',
    `description`    TEXT            NULL,
    `function_def`   JSON            NULL     COMMENT 'OpenAI function calling schema',
    `handler_type`   VARCHAR(32)     NOT NULL DEFAULT 'builtin' COMMENT 'builtin/http/script',
    `handler_config` JSON            NULL     COMMENT 'handler 配置',
    `enabled`        TINYINT(1)      NOT NULL DEFAULT 1,
    `timeout`        INT UNSIGNED    NOT NULL DEFAULT 30    COMMENT '执行超时(秒)',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 技能
CREATE TABLE IF NOT EXISTS `skills` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`        VARCHAR(36)     NOT NULL DEFAULT '',
    `name`        VARCHAR(128)    NOT NULL DEFAULT '',
    `description` TEXT            NULL,
    `instruction` TEXT            NULL     COMMENT '技能指令',
    `source`      VARCHAR(32)     NOT NULL DEFAULT 'custom'  COMMENT 'clawhub/local/custom',
    `slug`        VARCHAR(256)    NOT NULL DEFAULT ''         COMMENT 'ClawHub skill identifier',
    `version`     VARCHAR(32)     NOT NULL DEFAULT '',
    `author`      VARCHAR(128)    NOT NULL DEFAULT '',
    `dir_name`    VARCHAR(256)    NOT NULL DEFAULT ''         COMMENT 'workspace/skills/ 下的目录名',
    `main_file`   VARCHAR(128)    NOT NULL DEFAULT ''         COMMENT '可执行入口文件',
    `config`      JSON            NULL     COMMENT '配置 schema',
    `permissions` JSON            NULL     COMMENT '权限声明',
    `tool_defs`   JSON            NULL     COMMENT 'manifest 工具定义',
    `enabled`     TINYINT(1)      NOT NULL DEFAULT 1,
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- MCP 服务
CREATE TABLE IF NOT EXISTS `mcp_servers` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`        VARCHAR(36)     NOT NULL DEFAULT '',
    `name`        VARCHAR(128)    NOT NULL DEFAULT '',
    `description` TEXT            NULL,
    `transport`   VARCHAR(16)     NOT NULL DEFAULT 'stdio'   COMMENT 'stdio/sse',
    `endpoint`    VARCHAR(512)    NOT NULL DEFAULT ''         COMMENT 'command for stdio, URL for sse',
    `args`        JSON            NULL     COMMENT 'command args for stdio transport',
    `env`         JSON            NULL     COMMENT 'environment variables for stdio transport',
    `headers`     JSON            NULL     COMMENT 'HTTP headers for sse transport',
    `enabled`     TINYINT(1)      NOT NULL DEFAULT 1,
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 用户
CREATE TABLE IF NOT EXISTS `users` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `username`   VARCHAR(64)     NOT NULL DEFAULT '',
    `password`   VARCHAR(256)    NOT NULL DEFAULT '' COMMENT 'bcrypt hash',
    `role`       VARCHAR(16)     NOT NULL DEFAULT 'guest' COMMENT 'admin/guest',
    `enabled`    TINYINT(1)      NOT NULL DEFAULT 1,
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_username` (`username`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- ============================================================
--  关联表
-- ============================================================

-- Agent ↔ 工具
CREATE TABLE IF NOT EXISTS `agent_tools` (
    `agent_id` BIGINT UNSIGNED NOT NULL,
    `tool_id`  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`agent_id`, `tool_id`),
    KEY `idx_tool_id` (`tool_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- Agent ↔ 技能
CREATE TABLE IF NOT EXISTS `agent_skills` (
    `agent_id`  BIGINT UNSIGNED NOT NULL,
    `skill_id`  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`agent_id`, `skill_id`),
    KEY `idx_skill_id` (`skill_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- Agent ↔ MCP 服务
CREATE TABLE IF NOT EXISTS `agent_mcp_servers` (
    `agent_id`      BIGINT UNSIGNED NOT NULL,
    `mcp_server_id` BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`agent_id`, `mcp_server_id`),
    KEY `idx_mcp_server_id` (`mcp_server_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 技能 ↔ 工具
CREATE TABLE IF NOT EXISTS `skill_tools` (
    `skill_id` BIGINT UNSIGNED NOT NULL,
    `tool_id`  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`skill_id`, `tool_id`),
    KEY `idx_tool_id` (`tool_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- ============================================================
--  对话 & 消息 & 执行步骤
-- ============================================================

-- 会话
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
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 消息
CREATE TABLE IF NOT EXISTS `messages` (
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `conversation_id` BIGINT UNSIGNED NOT NULL,
    `role`            VARCHAR(16)     NOT NULL DEFAULT '' COMMENT 'system/user/assistant/tool',
    `content`         MEDIUMTEXT      NULL,
    `tool_calls`      JSON            NULL,
    `tool_call_id`    VARCHAR(64)     NOT NULL DEFAULT '',
    `tokens_used`     INT UNSIGNED    NOT NULL DEFAULT 0,
    `parent_step_id`  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联的执行步骤ID',
    `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_conversation_id` (`conversation_id`),
    KEY `idx_parent_step_id` (`parent_step_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 执行步骤
CREATE TABLE IF NOT EXISTS `execution_steps` (
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `message_id`      BIGINT UNSIGNED NOT NULL             COMMENT '关联的 assistant message ID',
    `conversation_id` BIGINT UNSIGNED NOT NULL,
    `step_order`      INT UNSIGNED    NOT NULL DEFAULT 0   COMMENT '步骤序号（从1开始）',
    `step_type`       VARCHAR(32)     NOT NULL DEFAULT ''  COMMENT 'planning/thinking/tool_call/reflection/memory_recall',
    `name`            VARCHAR(128)    NOT NULL DEFAULT ''   COMMENT '步骤名称',
    `input`           MEDIUMTEXT      NULL                 COMMENT '步骤输入内容',
    `output`          MEDIUMTEXT      NULL                 COMMENT '步骤输出内容',
    `status`          VARCHAR(16)     NOT NULL DEFAULT 'success' COMMENT 'success/error/pending',
    `error`           TEXT            NULL                 COMMENT '错误信息',
    `duration_ms`     INT UNSIGNED    NOT NULL DEFAULT 0   COMMENT '耗时(毫秒)',
    `tokens_used`     INT UNSIGNED    NOT NULL DEFAULT 0,
    `metadata`        JSON            NULL                 COMMENT '额外元数据',
    `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_message_id` (`message_id`),
    KEY `idx_conversation_id` (`conversation_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

-- 文件
CREATE TABLE IF NOT EXISTS `files` (
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid`            VARCHAR(36)     NOT NULL DEFAULT '',
    `conversation_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `message_id`      BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `filename`        VARCHAR(256)    NOT NULL DEFAULT '',
    `content_type`    VARCHAR(128)    NOT NULL DEFAULT '',
    `file_size`       BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `file_type`       VARCHAR(16)     NOT NULL DEFAULT 'text' COMMENT 'text/image/document',
    `storage_path`    VARCHAR(512)    NOT NULL DEFAULT '',
    `text_content`    MEDIUMTEXT      NULL     COMMENT '提取的文本内容',
    `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    KEY `idx_conversation` (`conversation_id`),
    KEY `idx_message` (`message_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
