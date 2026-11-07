-- vocab notebook
CREATE TABLE `vocab_card` (
  `id` INTEGER NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `uid` VARCHAR(64) NOT NULL UNIQUE,
  `creator_id` INTEGER NOT NULL,
  `term` VARCHAR(512) NOT NULL,
  `phonetic` VARCHAR(512) NOT NULL DEFAULT '',
  `definition` TEXT NOT NULL,
  `example` TEXT NOT NULL,
  `notes` TEXT NOT NULL,
  `tags` TEXT NOT NULL,
  `due_ts` BIGINT NOT NULL DEFAULT 0,
  `stability` DOUBLE NOT NULL DEFAULT 0,
  `difficulty` DOUBLE NOT NULL DEFAULT 0,
  `elapsed_days` INTEGER NOT NULL DEFAULT 0,
  `scheduled_days` INTEGER NOT NULL DEFAULT 0,
  `reps` INTEGER NOT NULL DEFAULT 0,
  `lapses` INTEGER NOT NULL DEFAULT 0,
  `state` INTEGER NOT NULL DEFAULT 0,
  `last_review_ts` BIGINT NOT NULL DEFAULT 0,
  `created_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  `updated_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
);

CREATE INDEX `idx_vocab_card_creator_due` ON `vocab_card`(`creator_id`, `due_ts`);
CREATE INDEX `idx_vocab_card_creator_term` ON `vocab_card`(`creator_id`, `term`);

CREATE TABLE `vocab_review_log` (
  `id` INTEGER NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `card_id` INTEGER NOT NULL,
  `rating` INTEGER NOT NULL,
  `state` INTEGER NOT NULL DEFAULT 0,
  `due_ts` BIGINT NOT NULL DEFAULT 0,
  `stability` DOUBLE NOT NULL DEFAULT 0,
  `difficulty` DOUBLE NOT NULL DEFAULT 0,
  `elapsed_days` INTEGER NOT NULL DEFAULT 0,
  `scheduled_days` INTEGER NOT NULL DEFAULT 0,
  `review_ts` BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
);

CREATE INDEX `idx_vocab_review_log_card_id` ON `vocab_review_log`(`card_id`);
