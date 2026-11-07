-- vocab notebook LLM settings
CREATE TABLE `vocab_setting` (
  `creator_id` INTEGER NOT NULL PRIMARY KEY,
  `enabled` TINYINT NOT NULL DEFAULT 0,
  `base_url` VARCHAR(1024) NOT NULL DEFAULT '',
  `api_key` TEXT NOT NULL,
  `model` VARCHAR(256) NOT NULL DEFAULT '',
  `prompt` TEXT NOT NULL,
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
);
