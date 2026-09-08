package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyAuthSnapshotDisabledMessage(t *testing.T) {
	id := int64(1)
	key := &APIKey{ID: 1, GroupID: &id, User: &User{ID: 1}, Group: &Group{ID: id, Hydrated: true, Status: "inactive", DisabledMessage: "请切换至 XXX 分组使用"}}
	svc := &APIKeyService{}
	data, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(data, &cached))
	restored, used, err := svc.applyAuthCacheEntry("test-key", &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.Equal(t, key.Group.DisabledMessage, restored.Group.DisabledMessage)
	cached.Snapshot.Version = 24
	_, used, err = svc.applyAuthCacheEntry("test-key", &cached)
	require.NoError(t, err)
	require.False(t, used)
}
