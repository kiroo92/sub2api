package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestPackageJobsPostgresOwnershipSnapshotAndRecovery(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	packages, db := packageTestService(t)
	ctx := context.Background()
	raw, err := migrations.FS.ReadFile("242_package_jobs.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO api_keys(id) VALUES(1),(2)`)
	require.NoError(t, err)
	plan, err := packages.SavePlan(ctx, testPackagePlan())
	require.NoError(t, err)
	for oid := int64(1); oid <= 2; oid++ {
		order, err := packageTestOrder(ctx, packages, plan, oid, 1, 0, time.Now().Add(-time.Hour))
		require.NoError(t, err)
		require.NoError(t, packages.fulfill(ctx, order))
	}
	owned, err := packages.ListOwned(ctx, 1)
	require.NoError(t, err)
	p := owned[0]
	keys := NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
	keys.SetPackageService(packages)
	key := &APIKey{ID: 1, UserID: 1, Key: "must-not-be-persisted", RoutingMode: APIKeyRoutingAllPackages,
		User: &User{ID: 1}, Group: &Group{ID: 1, RateMultiplier: 2, AccountGroups: []AccountGroup{{Account: &Account{Credentials: map[string]any{"token": "never-journal-account-secret"}}}}},
		PackageSelection: &PackageSelection{UserID: 1, GroupID: 1, PackageID: p.ID, PeriodID: p.CurrentPeriod.ID, RateMultiplier: 2}}
	job, err := keys.PreparePackageJob(ctx, key, "grok_video", 11)
	require.NoError(t, err)
	key.Group.RateMultiplier = 9
	key.PackageSelection.RateMultiplier = 9
	require.Equal(t, 2.0, job.Group.RateMultiplier)
	require.Equal(t, 2.0, job.Selection.RateMultiplier)
	job.VideoPending = &GrokVideoPendingBilling{Model: "grok-imagine-video", VideoDurationSeconds: 6, VideoResolution: "720p"}

	// Database writes fail after upstream creation. The returned resource and
	// original pricing/period survive, independently of request cancellation.
	_, err = db.Exec(`CREATE FUNCTION reject_package_completion() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test write outage'; END $$;
CREATE TRIGGER reject_package_completion BEFORE UPDATE ON package_jobs FOR EACH ROW EXECUTE FUNCTION reject_package_completion()`)
	require.NoError(t, err)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.NoError(t, keys.CompletePackageJob(canceled, key, job, "upstream-video-1"))
	path := packageJobRecoveryPath(1, 1, "grok_video", "upstream-video-1")
	raw, err = os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(raw), key.Key)
	require.NotContains(t, string(raw), "never-journal-account-secret")
	var saved PackageJob
	require.NoError(t, json.Unmarshal(raw, &saved))
	require.Equal(t, "upstream-video-1", saved.ResourceID)
	require.Equal(t, p.CurrentPeriod.ID, saved.Selection.PeriodID)
	require.Equal(t, job.VideoPending, saved.VideoPending)
	t.Run("another_replica_waits_for_recovery", func(t *testing.T) {
		t.Setenv("DATA_DIR", t.TempDir())
		_, err := keys.RestorePackageJob(ctx, key, "grok_video", saved.ResourceID)
		require.ErrorIs(t, err, ErrPackageSelectionRequired, "an owner draft must yield a retriable attribution error, not 404")
	})
	_, err = keys.RestorePackageJob(ctx, key, "grok_video", saved.ResourceID)
	require.Error(t, err, "an outage must leave recovery pending, never choose another package")
	require.FileExists(t, path)

	// A new service instance uses only persisted identity after reorder/expiry.
	_, err = db.Exec(`DROP TRIGGER reject_package_completion ON package_jobs;
UPDATE user_packages SET expires_at=NOW()-INTERVAL '1 second',sort_order=-id;
UPDATE package_periods SET used_usd=quota_usd`)
	require.NoError(t, err)
	restarted := NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
	restarted.SetPackageService(packages)
	bare := &APIKey{ID: 1, UserID: 1, RoutingMode: APIKeyRoutingAllPackages, User: &User{ID: 1}}
	restored, err := restarted.RestorePackageJob(ctx, bare, "grok_video", saved.ResourceID)
	require.NoError(t, err)
	require.Equal(t, p.ID, restored.PackageSelection.PackageID)
	require.Equal(t, p.CurrentPeriod.ID, restored.PackageSelection.PeriodID)
	require.Equal(t, int64(11), restored.PackageJob.AccountID)
	require.Equal(t, 2.0, restored.Group.RateMultiplier)
	require.Nil(t, bare.Group)
	require.NoFileExists(t, path)
	for _, foreign := range []*APIKey{
		{ID: 2, UserID: 1, RoutingMode: APIKeyRoutingAllPackages, User: &User{ID: 1}},
		{ID: 1, UserID: 2, RoutingMode: APIKeyRoutingAllPackages, User: &User{ID: 2}},
	} {
		_, err := restarted.RestorePackageJob(ctx, foreign, "grok_video", saved.ResourceID)
		require.ErrorIs(t, err, sql.ErrNoRows)
		require.Error(t, restarted.CompletePackageJob(ctx, foreign, job, saved.ResourceID))
	}
	_, err = restarted.RestorePackageJob(ctx, bare, "live", saved.ResourceID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	_, err = restarted.RestorePackageJob(ctx, bare, "grok_video", "unknown")
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Error(t, restarted.CompletePackageJob(ctx, key, job, "changed-resource"))
	require.NoError(t, restarted.CompletePackageJob(ctx, key, job, saved.ResourceID))
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM package_jobs`).Scan(&count))
	require.Equal(t, 1, count)

	// A process restart with an unflushed record is handled by the maintenance
	// sweep without a client poll and without calling an upstream provider.
	_, err = writePackageJobRecovery(&saved, raw)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE package_jobs SET resource_id=NULL`)
	require.NoError(t, err)
	require.NoError(t, packages.RecoverJobs(ctx))
	require.NoError(t, packages.RecoverJobs(ctx))
	require.NoFileExists(t, path)
	var resource string
	require.NoError(t, db.QueryRow(`SELECT resource_id FROM package_jobs`).Scan(&resource))
	require.Equal(t, saved.ResourceID, resource)

	// Live selects its account inside CreateLiveCall; completion must preserve
	// that account and call ID if the database goes away afterwards.
	live, err := keys.PreparePackageJob(ctx, key, "live", 0)
	require.NoError(t, err)
	live.AccountID = 32
	_, err = db.Exec(`CREATE TRIGGER reject_package_completion BEFORE UPDATE ON package_jobs FOR EACH ROW EXECUTE FUNCTION reject_package_completion()`)
	require.NoError(t, err)
	require.NoError(t, keys.CompletePackageJob(ctx, key, live, "upstream-live-1"))
	livePath := packageJobRecoveryPath(1, 1, "live", "upstream-live-1")
	require.FileExists(t, livePath)
	require.Error(t, packages.RecoverJobs(ctx))
	require.FileExists(t, livePath)
	_, err = db.Exec(`DROP TRIGGER reject_package_completion ON package_jobs`)
	require.NoError(t, err)
	require.NoError(t, packages.RecoverJobs(ctx))
	restored, err = restarted.RestorePackageJob(ctx, bare, "live", "upstream-live-1")
	require.NoError(t, err)
	require.Equal(t, int64(32), restored.PackageJob.AccountID)
	require.Equal(t, p.CurrentPeriod.ID, restored.PackageSelection.PeriodID)
	require.NoFileExists(t, livePath)
}

func TestPackageJobRecoveryPathAndIncompleteInput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	path := packageJobRecoveryPath(1, 2, "live", "../../escaped")
	require.Equal(t, filepath.Join(dir, "package-job-recovery"), filepath.Dir(path))
	keys := NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
	_, err := keys.PreparePackageJob(context.Background(), nil, "live", 0)
	require.ErrorIs(t, err, ErrPackageSelectionRequired)
	_, err = keys.RestorePackageJob(context.Background(), &APIKey{RoutingMode: APIKeyRoutingAllPackages}, "live", "id")
	require.ErrorIs(t, err, ErrPackageSelectionRequired)
	require.Error(t, keys.CompletePackageJob(context.Background(), nil, nil, ""))
}
