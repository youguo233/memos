-- vocab notebook LLM settings
CREATE TABLE vocab_setting (
  creator_id INTEGER PRIMARY KEY,
  enabled INTEGER NOT NULL DEFAULT 0,
  base_url TEXT NOT NULL DEFAULT '',
  api_key TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL DEFAULT '',
  updated_ts BIGINT NOT NULL DEFAULT (strftime('%s', 'now'))
);
