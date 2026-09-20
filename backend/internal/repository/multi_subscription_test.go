package repository

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMultiSubscriptionCreateReorderAndFixedGroupIsolation(t *testing.T) {
	_, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	u := mustCreateAPIKeyRepoUser(t, ctx, client, "multi@example.com")
	g, err := client.Group.Create().SetName("multi").SetSubscriptionType(service.SubscriptionTypeSubscription).Save(ctx)
	require.NoError(t, err)
	r, ok := NewUserSubscriptionRepository(client).(*userSubscriptionRepository)
	require.True(t, ok)
	create := func() *service.UserSubscription {
		sub := &service.UserSubscription{UserID: u.ID, GroupID: g.ID, StartsAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Status: service.SubscriptionStatusActive}
		require.NoError(t, r.Create(ctx, sub))
		return sub
	}
	first, second := create(), create()
	require.NotEqual(t, first.ID, second.ID)
	require.Less(t, first.SortOrder, second.SortOrder)
	fixed, err := r.GetActiveByUserIDAndGroupID(ctx, u.ID, g.ID)
	require.NoError(t, err)
	require.NoError(t, r.ReorderActive(ctx, u.ID, []int64{second.ID, first.ID}))
	ordered, err := r.ListActiveByUserID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{second.ID, first.ID}, []int64{ordered[0].ID, ordered[1].ID})
	after, err := r.GetActiveByUserIDAndGroupID(ctx, u.ID, g.ID)
	require.NoError(t, err)
	require.Equal(t, fixed.ID, after.ID, "reordering cannot change fixed-group consumption")
	require.ErrorIs(t, r.ReorderActive(ctx, u.ID, []int64{first.ID, first.ID}), service.ErrSubscriptionOrderConflict)
	require.ErrorIs(t, r.ReorderActive(ctx, u.ID, []int64{first.ID, 99999}), service.ErrSubscriptionOrderConflict)
	ordered, err = r.ListActiveByUserID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, second.ID, ordered[0].ID, "rejected reorder rolls back earlier row writes")
	third := create()
	require.Greater(t, third.SortOrder, ordered[1].SortOrder)
	// Repository constructors can receive an already transactional Ent client.
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	txRepo := NewUserSubscriptionRepository(tx.Client())
	sub := &service.UserSubscription{UserID: u.ID, GroupID: g.ID, StartsAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, txRepo.Create(dbent.NewTxContext(ctx, tx), sub))
}
