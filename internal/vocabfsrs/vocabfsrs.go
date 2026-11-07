// Package vocabfsrs wraps open-spaced-repetition/go-fsrs for the vocabulary notebook.
package vocabfsrs

import (
	"fmt"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v3"
	"github.com/usememos/memos/store"
)

// Rating values match Anki/FSRS: Again=1, Hard=2, Good=3, Easy=4.

var scheduler = fsrs.NewFSRS(fsrs.DefaultParam())

// Preview is the scheduled outcome of one rating, used to label review buttons.
type Preview struct {
	DueTs         int64
	ScheduledDays int64
}

func toFSRS(c *store.VocabCard) fsrs.Card {
	card := fsrs.Card{
		Stability:     c.Stability,
		Difficulty:    c.Difficulty,
		ElapsedDays:   uint64(max64(c.ElapsedDays, 0)),
		ScheduledDays: uint64(max64(c.ScheduledDays, 0)),
		Reps:          uint64(max32(c.Reps, 0)),
		Lapses:        uint64(max32(c.Lapses, 0)),
		State:         fsrs.State(c.State),
	}
	if c.DueTs > 0 {
		card.Due = time.Unix(c.DueTs, 0).UTC()
	}
	if c.LastReviewTs > 0 {
		card.LastReview = time.Unix(c.LastReviewTs, 0).UTC()
	}
	return card
}

func applyFSRS(c *store.VocabCard, card fsrs.Card) {
	c.DueTs = card.Due.Unix()
	c.Stability = card.Stability
	c.Difficulty = card.Difficulty
	c.ElapsedDays = int64(card.ElapsedDays)
	c.ScheduledDays = int64(card.ScheduledDays)
	c.Reps = int32(card.Reps)
	c.Lapses = int32(card.Lapses)
	c.State = int32(card.State)
	if !card.LastReview.IsZero() {
		c.LastReviewTs = card.LastReview.Unix()
	}
}

// PreviewRatings returns the four possible next reviews without changing the card.
func PreviewRatings(c *store.VocabCard, now time.Time) map[int]Preview {
	record := scheduler.Repeat(toFSRS(c), now.UTC())
	out := make(map[int]Preview, 4)
	for rating := fsrs.Again; rating <= fsrs.Easy; rating++ {
		info := record[rating]
		out[int(rating)] = Preview{
			DueTs:         info.Card.Due.Unix(),
			ScheduledDays: int64(info.Card.ScheduledDays),
		}
	}
	return out
}

// ApplyRating updates card in place and returns a review log snapshot of the new state.
func ApplyRating(c *store.VocabCard, rating int, now time.Time) (*store.VocabReviewLog, error) {
	if rating < int(fsrs.Again) || rating > int(fsrs.Easy) {
		return nil, fmt.Errorf("rating must be 1-4")
	}
	info := scheduler.Next(toFSRS(c), now.UTC(), fsrs.Rating(rating))
	applyFSRS(c, info.Card)
	if c.LastReviewTs == 0 {
		c.LastReviewTs = now.Unix()
	}
	return &store.VocabReviewLog{
		CardID:        c.ID,
		Rating:        int32(rating),
		State:         c.State,
		DueTs:         c.DueTs,
		Stability:     c.Stability,
		Difficulty:    c.Difficulty,
		ElapsedDays:   c.ElapsedDays,
		ScheduledDays: c.ScheduledDays,
		ReviewTs:      now.Unix(),
	}, nil
}

func max64(v, floor int64) int64 {
	if v < floor {
		return floor
	}
	return v
}

func max32(v, floor int32) int32 {
	if v < floor {
		return floor
	}
	return v
}
