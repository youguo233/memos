package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/store"
)

func TestUserPasskeyCRUD(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)
	p, err := ts.CreateUserPasskey(ctx, &store.UserPasskey{UserID: user.ID, CredentialID: "abc", Name: "Mac", Credential: "{}"})
	require.NoError(t, err)
	require.NotZero(t, p.ID)
	cid := "abc"
	got, err := ts.GetUserPasskey(ctx, &store.FindUserPasskey{CredentialID: &cid})
	require.NoError(t, err)
	require.Equal(t, "Mac", got.Name)
	require.NoError(t, ts.UpdateUserPasskeyUsage(ctx, p.ID, `{"x":1}`))
	list, err := ts.ListUserPasskeys(ctx, &store.FindUserPasskey{UserID: &user.ID})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotZero(t, list[0].LastUsedTs)
	ok, err := ts.DeleteUserPasskey(ctx, user.ID+1, p.ID)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = ts.DeleteUserPasskey(ctx, user.ID, p.ID)
	require.NoError(t, err)
	require.True(t, ok)
}
