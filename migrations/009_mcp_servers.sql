CREATE TABLE IF NOT EXISTS `mcp_servers` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `uuid` VARCHAR(36) NOT NULL DEFAULT '',
    `name` VARCHAR(128) NOT NULL DEFAULT '',
    `description` TEXT NULL,
    `transport` VARCHAR(16) NOT NULL DEFAULT 'stdio' COMMENT 'stdio/sse',
    `endpoint` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'command for stdio, URL for sse',
    `args` JSON NULL COMMENT 'command args for stdio transport',
    `env` JSON NULL COMMENT 'environment variables for stdio transport',
    `headers` JSON NULL COMMENT 'HTTP headers for sse transport',
    `enabled` TINYINT(1) NOT NULL DEFAULT 1,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_uuid` (`uuid`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `agent_mcp_servers` (
    `agent_id` BIGINT UNSIGNED NOT NULL,
    `mcp_server_id` BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`agent_id`, `mcp_server_id`),
    KEY `idx_mcp_server_id` (`mcp_server_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
