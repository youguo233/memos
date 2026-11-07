package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Vocab card states match FSRS: New=0, Learning=1, Review=2, Relearning=3.

// VocabCard is one word or phrase in a user's notebook.
type VocabCard struct {
	ID            int32
	UID           string
	CreatorID     int32
	Term          string
	Phonetic      string
	Definition    string
	Example       string
	Notes         string
	Tags          string
	DueTs         int64
	Stability     float64
	Difficulty    float64
	ElapsedDays   int64
	ScheduledDays int64
	Reps          int32
	Lapses        int32
	State         int32
	LastReviewTs  int64
	CreatedTs     int64
	UpdatedTs     int64
}

// VocabReviewLog is one rating applied to a card.
type VocabReviewLog struct {
	ID            int32
	CardID        int32
	Rating        int32
	State         int32
	DueTs         int64
	Stability     float64
	Difficulty    float64
	ElapsedDays   int64
	ScheduledDays int64
	ReviewTs      int64
}

// FindVocabCard filters vocabulary cards.
type FindVocabCard struct {
	ID            *int32
	UID           *string
	CreatorID     *int32
	TermEqualFold *string
	Query         *string
	// Filter: "", "all", "due", "due_review", "new", "learning", "review"
	Filter    string
	Sort      string
	DueBefore *int64
	Limit     int
	Offset    int
}

// VocabStats counts a user's cards.
type VocabStats struct {
	Total    int
	New      int
	Learning int
	Review   int
	Due      int
}

const vocabCardColumns = `id, uid, creator_id, term, phonetic, definition, example, notes, tags,
due_ts, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review_ts, created_ts, updated_ts`

func scanVocabCard(scanner interface{ Scan(dest ...any) error }) (*VocabCard, error) {
	c := &VocabCard{}
	err := scanner.Scan(
		&c.ID, &c.UID, &c.CreatorID, &c.Term, &c.Phonetic, &c.Definition, &c.Example, &c.Notes, &c.Tags,
		&c.DueTs, &c.Stability, &c.Difficulty, &c.ElapsedDays, &c.ScheduledDays, &c.Reps, &c.Lapses, &c.State, &c.LastReviewTs, &c.CreatedTs, &c.UpdatedTs,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) CreateVocabCard(ctx context.Context, c *VocabCard) (*VocabCard, error) {
	now := time.Now().Unix()
	if c.CreatedTs == 0 {
		c.CreatedTs = now
	}
	c.UpdatedTs = now
	if c.DueTs == 0 {
		c.DueTs = now
	}
	db := s.driver.GetDB()
	if s.profile != nil && s.profile.Driver == "postgres" {
		q := s.rebind(`INSERT INTO vocab_card (
			uid, creator_id, term, phonetic, definition, example, notes, tags,
			due_ts, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review_ts, created_ts, updated_ts
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`)
		if err := db.QueryRowContext(ctx, q,
			c.UID, c.CreatorID, c.Term, c.Phonetic, c.Definition, c.Example, c.Notes, c.Tags,
			c.DueTs, c.Stability, c.Difficulty, c.ElapsedDays, c.ScheduledDays, c.Reps, c.Lapses, c.State, c.LastReviewTs, c.CreatedTs, c.UpdatedTs,
		).Scan(&c.ID); err != nil {
			return nil, err
		}
		return c, nil
	}
	res, err := db.ExecContext(ctx, `INSERT INTO vocab_card (
		uid, creator_id, term, phonetic, definition, example, notes, tags,
		due_ts, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review_ts, created_ts, updated_ts
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.UID, c.CreatorID, c.Term, c.Phonetic, c.Definition, c.Example, c.Notes, c.Tags,
		c.DueTs, c.Stability, c.Difficulty, c.ElapsedDays, c.ScheduledDays, c.Reps, c.Lapses, c.State, c.LastReviewTs, c.CreatedTs, c.UpdatedTs,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	c.ID = int32(id)
	return c, nil
}

func (s *Store) vocabFindSQL(find *FindVocabCard) (string, []any) {
	where, args := []string{"1 = 1"}, []any{}
	if find.ID != nil {
		where, args = append(where, "id = ?"), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, "uid = ?"), append(args, *find.UID)
	}
	if find.CreatorID != nil {
		where, args = append(where, "creator_id = ?"), append(args, *find.CreatorID)
	}
	if find.TermEqualFold != nil {
		where, args = append(where, "LOWER(term) = LOWER(?)"), append(args, *find.TermEqualFold)
	}
	if find.Query != nil && strings.TrimSpace(*find.Query) != "" {
		like := "%" + strings.TrimSpace(*find.Query) + "%"
		where = append(where, "(LOWER(term) LIKE LOWER(?) OR LOWER(definition) LIKE LOWER(?) OR LOWER(example) LIKE LOWER(?) OR LOWER(notes) LIKE LOWER(?) OR LOWER(tags) LIKE LOWER(?))")
		args = append(args, like, like, like, like, like)
	}
	dueBefore := time.Now().Unix()
	if find.DueBefore != nil {
		dueBefore = *find.DueBefore
	}
	switch find.Filter {
	case "due":
		where, args = append(where, "due_ts <= ?"), append(args, dueBefore)
	case "due_review":
		where, args = append(where, "state != 0 AND due_ts <= ?"), append(args, dueBefore)
	case "new":
		where = append(where, "state = 0")
	case "learning":
		where = append(where, "state IN (1, 3)")
	case "review":
		where = append(where, "state = 2")
	}
	order := "created_ts DESC, id DESC"
	switch find.Sort {
	case "due":
		order = "due_ts ASC, id ASC"
	case "term":
		order = "LOWER(term) ASC, id ASC"
	case "added":
		order = "created_ts DESC, id DESC"
	}
	q := fmt.Sprintf("SELECT %s FROM vocab_card WHERE %s ORDER BY %s", vocabCardColumns, strings.Join(where, " AND "), order)
	if find.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", find.Limit)
		if find.Offset > 0 {
			q += fmt.Sprintf(" OFFSET %d", find.Offset)
		}
	}
	return s.rebind(q), args
}

func (s *Store) ListVocabCards(ctx context.Context, find *FindVocabCard) ([]*VocabCard, error) {
	if find == nil {
		find = &FindVocabCard{}
	}
	q, args := s.vocabFindSQL(find)
	rows, err := s.driver.GetDB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*VocabCard
	for rows.Next() {
		c, err := scanVocabCard(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) GetVocabCard(ctx context.Context, find *FindVocabCard) (*VocabCard, error) {
	list, err := s.ListVocabCards(ctx, find)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[0], nil
}

// UpdateVocabCardContent updates editable fields (not the scheduler state).
func (s *Store) UpdateVocabCardContent(ctx context.Context, c *VocabCard) error {
	c.UpdatedTs = time.Now().Unix()
	_, err := s.driver.GetDB().ExecContext(ctx, s.rebind(`UPDATE vocab_card SET
		term = ?, phonetic = ?, definition = ?, example = ?, notes = ?, tags = ?, updated_ts = ?
		WHERE id = ? AND creator_id = ?`),
		c.Term, c.Phonetic, c.Definition, c.Example, c.Notes, c.Tags, c.UpdatedTs, c.ID, c.CreatorID,
	)
	return err
}

// UpdateVocabCardScheduler persists FSRS fields after a review.
func (s *Store) UpdateVocabCardScheduler(ctx context.Context, c *VocabCard) error {
	c.UpdatedTs = time.Now().Unix()
	_, err := s.driver.GetDB().ExecContext(ctx, s.rebind(`UPDATE vocab_card SET
		due_ts = ?, stability = ?, difficulty = ?, elapsed_days = ?, scheduled_days = ?,
		reps = ?, lapses = ?, state = ?, last_review_ts = ?, updated_ts = ?
		WHERE id = ? AND creator_id = ?`),
		c.DueTs, c.Stability, c.Difficulty, c.ElapsedDays, c.ScheduledDays,
		c.Reps, c.Lapses, c.State, c.LastReviewTs, c.UpdatedTs, c.ID, c.CreatorID,
	)
	return err
}

func (s *Store) DeleteVocabCard(ctx context.Context, creatorID int32, uid string) (bool, error) {
	card, err := s.GetVocabCard(ctx, &FindVocabCard{CreatorID: &creatorID, UID: &uid})
	if err != nil {
		return false, err
	}
	if card == nil {
		return false, nil
	}
	if _, err := s.driver.GetDB().ExecContext(ctx, s.rebind("DELETE FROM vocab_review_log WHERE card_id = ?"), card.ID); err != nil {
		return false, err
	}
	res, err := s.driver.GetDB().ExecContext(ctx, s.rebind("DELETE FROM vocab_card WHERE id = ? AND creator_id = ?"), card.ID, creatorID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) CreateVocabReviewLog(ctx context.Context, log *VocabReviewLog) (*VocabReviewLog, error) {
	if log.ReviewTs == 0 {
		log.ReviewTs = time.Now().Unix()
	}
	db := s.driver.GetDB()
	if s.profile != nil && s.profile.Driver == "postgres" {
		q := s.rebind(`INSERT INTO vocab_review_log (
			card_id, rating, state, due_ts, stability, difficulty, elapsed_days, scheduled_days, review_ts
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`)
		if err := db.QueryRowContext(ctx, q, log.CardID, log.Rating, log.State, log.DueTs, log.Stability, log.Difficulty, log.ElapsedDays, log.ScheduledDays, log.ReviewTs).Scan(&log.ID); err != nil {
			return nil, err
		}
		return log, nil
	}
	res, err := db.ExecContext(ctx, `INSERT INTO vocab_review_log (
		card_id, rating, state, due_ts, stability, difficulty, elapsed_days, scheduled_days, review_ts
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.CardID, log.Rating, log.State, log.DueTs, log.Stability, log.Difficulty, log.ElapsedDays, log.ScheduledDays, log.ReviewTs,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	log.ID = int32(id)
	return log, nil
}

func (s *Store) ListVocabReviewLogs(ctx context.Context, cardID int32, limit int) ([]*VocabReviewLog, error) {
	if limit <= 0 {
		limit = 20
	}
	q := s.rebind(fmt.Sprintf(`SELECT id, card_id, rating, state, due_ts, stability, difficulty, elapsed_days, scheduled_days, review_ts
		FROM vocab_review_log WHERE card_id = ? ORDER BY review_ts DESC, id DESC LIMIT %d`, limit))
	rows, err := s.driver.GetDB().QueryContext(ctx, q, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*VocabReviewLog
	for rows.Next() {
		l := &VocabReviewLog{}
		if err := rows.Scan(&l.ID, &l.CardID, &l.Rating, &l.State, &l.DueTs, &l.Stability, &l.Difficulty, &l.ElapsedDays, &l.ScheduledDays, &l.ReviewTs); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (s *Store) GetVocabStats(ctx context.Context, creatorID int32, now int64) (*VocabStats, error) {
	q := s.rebind(`SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN state = 0 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state IN (1, 3) THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state = 2 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state != 0 AND due_ts <= ? THEN 1 ELSE 0 END), 0)
		FROM vocab_card WHERE creator_id = ?`)
	st := &VocabStats{}
	if err := s.driver.GetDB().QueryRowContext(ctx, q, now, creatorID).Scan(&st.Total, &st.New, &st.Learning, &st.Review, &st.Due); err != nil {
		return nil, err
	}
	return st, nil
}
