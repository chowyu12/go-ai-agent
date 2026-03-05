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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
