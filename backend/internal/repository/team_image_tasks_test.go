package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestDeleteTeamImageTaskCachePreservesPersonalTasks(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := NewImageTaskStore(rdb)
	ctx := context.Background()
	require.NoError(t, store.Save(ctx, &service.ImageTaskRecord{ID: "team", APIKeyID: 1}, time.Hour))
	require.NoError(t, store.Save(ctx, &service.ImageTaskRecord{ID: "personal", APIKeyID: 2}, time.Hour))
	require.NoError(t, deleteTeamImageTaskCache(ctx, rdb, map[int64]bool{1: true}))
	_, err := store.Get(ctx, "team")
	require.ErrorIs(t, err, service.ErrImageTaskNotFound)
	_, err = store.Get(ctx, "personal")
	require.NoError(t, err)
}
