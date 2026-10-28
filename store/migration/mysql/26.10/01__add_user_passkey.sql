CREATE TABLE `user_passkey` (
  `id`            INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `user_id`       INT          NOT NULL,
  `credential_id` VARCHAR(512) NOT NULL UNIQUE,
  `name`          VARCHAR(256) NOT NULL DEFAULT '',
  `credential`    LONGTEXT     NOT NULL,
  `created_ts`    BIGINT       NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `last_used_ts`  BIGINT       NOT NULL DEFAULT 0
);

CREATE INDEX `idx_user_passkey_user_id` ON `user_passkey`(`user_id`);
