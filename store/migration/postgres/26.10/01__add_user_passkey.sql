CREATE TABLE user_passkey (
  id            SERIAL  PRIMARY KEY,
  user_id       INTEGER NOT NULL,
  credential_id TEXT    NOT NULL UNIQUE,
  name          TEXT    NOT NULL DEFAULT '',
  credential    TEXT    NOT NULL,
  created_ts    BIGINT  NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW()),
  last_used_ts  BIGINT  NOT NULL DEFAULT 0
);

CREATE INDEX idx_user_passkey_user_id ON user_passkey(user_id);
