ALTER TABLE `skills`
    ADD COLUMN `source` VARCHAR(32) NOT NULL DEFAULT 'custom' COMMENT 'clawhub/local/custom' AFTER `instruction`,
    ADD COLUMN `slug` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'ClawHub skill identifier' AFTER `source`,
    ADD COLUMN `version` VARCHAR(32) NOT NULL DEFAULT '' AFTER `slug`,
    ADD COLUMN `author` VARCHAR(128) NOT NULL DEFAULT '' AFTER `version`,
    ADD COLUMN `dir_name` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'workspace/skills/ 下的目录名' AFTER `author`,
    ADD COLUMN `main_file` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '可执行入口文件' AFTER `dir_name`,
    ADD COLUMN `config` JSON NULL COMMENT '配置 schema' AFTER `main_file`,
    ADD COLUMN `permissions` JSON NULL COMMENT '权限声明' AFTER `config`,
    ADD COLUMN `tool_defs` JSON NULL COMMENT 'manifest 工具定义' AFTER `permissions`,
    ADD COLUMN `enabled` TINYINT(1) NOT NULL DEFAULT 1 AFTER `tool_defs`;
