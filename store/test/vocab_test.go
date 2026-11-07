package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/store"
)

func TestVocabCardCRUDAndIsolation(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	owner, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)
	other, err := createTestingUserWithRole(ctx, ts, "other", store.RoleUser)
	require.NoError(t, err)

	card, err := ts.CreateVocabCard(ctx, &store.VocabCard{
		UID:        "uid-affect",
		CreatorID:  owner.ID,
		Term:       "affect",
		Phonetic:   "/əˈfekt/",
		Definition: "to influence",
		Example:    "The result may affect the conclusion.",
		Notes:      "from a paper",
		Tags:       `["ML"]`,
	})
	require.NoError(t, err)
	require.NotZero(t, card.ID)
	require.NotZero(t, card.DueTs)

	got, err := ts.GetVocabCard(ctx, &store.FindVocabCard{UID: strPtr("uid-affect"), CreatorID: &owner.ID})
	require.NoError(t, err)
	require.Equal(t, "affect", got.Term)

	term := "affect"
	dup, err := ts.GetVocabCard(ctx, &store.FindVocabCard{CreatorID: &owner.ID, TermEqualFold: &term})
	require.NoError(t, err)
	require.Equal(t, card.ID, dup.ID)

	hidden, err := ts.GetVocabCard(ctx, &store.FindVocabCard{UID: strPtr("uid-affect"), CreatorID: &other.ID})
	require.NoError(t, err)
	require.Nil(t, hidden)

	q := "influence"
	found, err := ts.ListVocabCards(ctx, &store.FindVocabCard{CreatorID: &owner.ID, Query: &q})
	require.NoError(t, err)
	require.Len(t, found, 1)

	now := time.Now().Unix()
	due, err := ts.ListVocabCards(ctx, &store.FindVocabCard{CreatorID: &owner.ID, Filter: "due", DueBefore: &now})
	require.NoError(t, err)
	require.Len(t, due, 1)

	news, err := ts.ListVocabCards(ctx, &store.FindVocabCard{CreatorID: &owner.ID, Filter: "new"})
	require.NoError(t, err)
	require.Len(t, news, 1)

	card.Definition = "to produce a change in"
	require.NoError(t, ts.UpdateVocabCardContent(ctx, card))
	card.State = 2
	card.DueTs = now + 86400
	require.NoError(t, ts.UpdateVocabCardScheduler(ctx, card))

	log, err := ts.CreateVocabReviewLog(ctx, &store.VocabReviewLog{CardID: card.ID, Rating: 3, State: 2, DueTs: card.DueTs, ReviewTs: now})
	require.NoError(t, err)
	require.NotZero(t, log.ID)
	logs, err := ts.ListVocabReviewLogs(ctx, card.ID, 10)
	require.NoError(t, err)
	require.Len(t, logs, 1)

	stats, err := ts.GetVocabStats(ctx, owner.ID, now)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Total)
	require.Equal(t, 0, stats.New)
	require.Equal(t, 1, stats.Review)
	require.Equal(t, 0, stats.Due)

	otherStats, err := ts.GetVocabStats(ctx, other.ID, now)
	require.NoError(t, err)
	require.Equal(t, 0, otherStats.Total)

	ok, err := ts.DeleteVocabCard(ctx, other.ID, "uid-affect")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = ts.DeleteVocabCard(ctx, owner.ID, "uid-affect")
	require.NoError(t, err)
	require.True(t, ok)
	logs, err = ts.ListVocabReviewLogs(ctx, card.ID, 10)
	require.NoError(t, err)
	require.Empty(t, logs)
}

func strPtr(s string) *string { return &s }

func TestVocabSettingUpsertAndIsolation(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()

	owner, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)
	other, err := createTestingUserWithRole(ctx, ts, "other", store.RoleUser)
	require.NoError(t, err)

	missing, err := ts.GetVocabSetting(ctx, owner.ID)
	require.NoError(t, err)
	require.Nil(t, missing)

	require.NoError(t, ts.UpsertVocabSetting(ctx, &store.VocabSetting{
		CreatorID: owner.ID,
		Enabled:   true,
		BaseURL:   "https://api.deepseek.com/v1",
		APIKey:    "sk-test",
		Model:     "deepseek-chat",
		Prompt:    "custom",
	}))
	got, err := ts.GetVocabSetting(ctx, owner.ID)
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, "https://api.deepseek.com/v1", got.BaseURL)
	require.Equal(t, "sk-test", got.APIKey)
	require.Equal(t, "deepseek-chat", got.Model)
	require.Equal(t, "custom", got.Prompt)

	hidden, err := ts.GetVocabSetting(ctx, other.ID)
	require.NoError(t, err)
	require.Nil(t, hidden)

	require.NoError(t, ts.UpsertVocabSetting(ctx, &store.VocabSetting{
		CreatorID: owner.ID,
		Enabled:   false,
		BaseURL:   "https://api.openai.com/v1",
		APIKey:    "sk-new",
		Model:     "gpt-4o-mini",
	}))
	got, err = ts.GetVocabSetting(ctx, owner.ID)
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.Equal(t, "sk-new", got.APIKey)
	require.Equal(t, "gpt-4o-mini", got.Model)
	require.Equal(t, "", got.Prompt)
}
