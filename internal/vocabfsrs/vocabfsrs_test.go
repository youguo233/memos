package vocabfsrs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestPreviewAndReview(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	card := &store.VocabCard{ID: 1, Term: "affect", State: 0, DueTs: now.Unix()}
	previews := PreviewRatings(card, now)
	require.Len(t, previews, 4)
	require.Contains(t, previews, 1)
	require.Contains(t, previews, 4)

	againCard := *card
	againLog, err := ApplyRating(&againCard, 1, now)
	require.NoError(t, err)
	require.Equal(t, int32(1), againLog.Rating)
	require.NotZero(t, againCard.DueTs)

	goodCard := *card
	goodLog, err := ApplyRating(&goodCard, 3, now)
	require.NoError(t, err)
	require.Equal(t, int32(3), goodLog.Rating)
	require.Greater(t, goodCard.DueTs, againCard.DueTs)
	require.NotEqual(t, int32(0), goodCard.State)

	_, err = ApplyRating(card, 9, now)
	require.Error(t, err)
}
