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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
