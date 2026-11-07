package store

import (
	"context"
	"database/sql"
	"time"
)

// VocabSetting stores a user's OpenAI-compatible lookup API.
type VocabSetting struct {
	CreatorID int32
	Enabled   bool
	BaseURL   string
	APIKey    string
	Model     string
	Prompt    string
	UpdatedTs int64
}

func (s *Store) GetVocabSetting(ctx context.Context, creatorID int32) (*VocabSetting, error) {
	row := s.driver.GetDB().QueryRowContext(ctx, s.rebind(`SELECT creator_id, enabled, base_url, api_key, model, prompt, updated_ts
		FROM vocab_setting WHERE creator_id = ?`), creatorID)
	st := &VocabSetting{}
	var enabled int
	if err := row.Scan(&st.CreatorID, &enabled, &st.BaseURL, &st.APIKey, &st.Model, &st.Prompt, &st.UpdatedTs); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	st.Enabled = enabled != 0
	return st, nil
}

func (s *Store) UpsertVocabSetting(ctx context.Context, st *VocabSetting) error {
	st.UpdatedTs = time.Now().Unix()
	enabled := 0
	if st.Enabled {
		enabled = 1
	}
	db := s.driver.GetDB()
	if _, err := db.ExecContext(ctx, s.rebind(`DELETE FROM vocab_setting WHERE creator_id = ?`), st.CreatorID); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, s.rebind(`INSERT INTO vocab_setting (
		creator_id, enabled, base_url, api_key, model, prompt, updated_ts
	) VALUES (?, ?, ?, ?, ?, ?, ?)`),
		st.CreatorID, enabled, st.BaseURL, st.APIKey, st.Model, st.Prompt, st.UpdatedTs,
	)
	return err
}
