//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestAdminGroupDisabledMessage(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 1}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{groupRepo: repo, authCacheInvalidator: invalidator}
	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "test", Platform: PlatformOpenAI, RateMultiplier: 1, DisabledMessage: "  请切换至 XXX  "})
	require.NoError(t, err)
	require.Equal(t, "请切换至 XXX", repo.created.DisabledMessage)
	require.Equal(t, group.DisabledMessage, cloneGroupForDuplicate(group, "test-copy").DisabledMessage)
	repo.getByID = group
	_, err = svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{})
	require.NoError(t, err)
	require.Equal(t, "请切换至 XXX", repo.updated.DisabledMessage)
	msg := "新的停用提示"
	_, err = svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{DisabledMessage: &msg})
	require.NoError(t, err)
	require.Equal(t, msg, repo.updated.DisabledMessage)
	msg = "   "
	_, err = svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{DisabledMessage: &msg})
	require.NoError(t, err)
	require.Empty(t, repo.updated.DisabledMessage)
	require.Equal(t, []int64{group.ID, group.ID, group.ID}, invalidator.groupIDs)
	msg = strings.Repeat("字", 1001)
	_, err = svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{DisabledMessage: &msg})
	require.Error(t, err)
	_, err = svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "test", Platform: PlatformOpenAI, RateMultiplier: 1, DisabledMessage: msg})
	require.Error(t, err)
}
