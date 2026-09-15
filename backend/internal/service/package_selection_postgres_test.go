package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

type packageSelectionAccounts struct{ AccountRepository }

func (packageSelectionAccounts) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return []Account{{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive}}, nil
}

func TestPackagePostgresSelectorOrderExhaustionAndIsolation(t *testing.T) {
	packages, db := packageTestService(t)
	ctx := context.Background()
	raw, err := migrations.FS.ReadFile("241_package_usage_billing.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	plan, err := packages.SavePlan(ctx, testPackagePlan())
	require.NoError(t, err)
	for oid := int64(1); oid <= 2; oid++ {
		order, err := packageTestOrder(ctx, packages, plan, oid, 1, 0, time.Now().Add(-time.Hour))
		require.NoError(t, err)
		require.NoError(t, packages.fulfill(ctx, order))
	}
	keys := NewAPIKeyService(nil, nil, packageTestGroups{}, nil, nil, nil, nil)
	keys.SetPackageService(packages)
	keys.SetPackageRoutingRepositories(packageSelectionAccounts{}, nil)
	override := 0
	key := &APIKey{ID: 1, UserID: 1, RoutingMode: APIKeyRoutingAllPackages, User: &User{ID: 1, UserGroupRPMOverride: &override}}
	req := PackageRequest{Path: "/v1/responses", Models: []string{"gpt-5"}}
	first, err := keys.SelectPackage(ctx, key, req)
	require.NoError(t, err)
	require.NotNil(t, first.PackageSelection)
	require.Nil(t, key.PackageSelection)
	require.Nil(t, key.Group)
	require.Same(t, &override, key.User.UserGroupRPMOverride)
	require.Nil(t, first.User.UserGroupRPMOverride)
	_, err = db.Exec(`UPDATE package_periods SET used_usd=quota_usd WHERE id=$1`, first.PackageSelection.PeriodID)
	require.NoError(t, err)
	second, err := keys.SelectPackage(ctx, key, req)
	require.NoError(t, err)
	require.NotEqual(t, first.PackageSelection.PackageID, second.PackageSelection.PackageID)
	_, err = db.Exec(`UPDATE package_periods SET used_usd=0 WHERE id=$1`, first.PackageSelection.PeriodID)
	require.NoError(t, err)
	restored, err := keys.SelectPackage(ctx, key, req)
	require.NoError(t, err)
	require.Equal(t, first.PackageSelection.PackageID, restored.PackageSelection.PackageID)
	require.NoError(t, packages.Reorder(ctx, 1, []int64{second.PackageSelection.PackageID, first.PackageSelection.PackageID}))
	reordered, err := keys.SelectPackage(ctx, key, req)
	require.NoError(t, err)
	require.Equal(t, second.PackageSelection.PackageID, reordered.PackageSelection.PackageID)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			selected, err := keys.SelectPackage(ctx, key, req)
			if err == nil {
				selected.User.Balance = 999
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	require.Zero(t, key.User.Balance)
	require.Nil(t, key.GroupID)
	_, err = db.Exec(`UPDATE package_periods SET used_usd=quota_usd`)
	require.NoError(t, err)
	_, err = keys.SelectPackage(ctx, key, req)
	require.ErrorIs(t, err, ErrNoUsablePackage)
	groups, err := keys.PackageGroups(ctx, key)
	require.NoError(t, err)
	require.Len(t, groups, 1, "exhausted active packages remain discoverable")
}
