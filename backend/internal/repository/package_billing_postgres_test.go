package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func packageBillingTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PACKAGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PACKAGE_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("package_billing_test_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(12)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		_, err := base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, err)
		require.NoError(t, base.Close())
	})
	_, err = db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,8) DEFAULT 100);
 CREATE TABLE api_keys(id BIGINT PRIMARY KEY,user_id BIGINT,quota NUMERIC(20,8) DEFAULT 100,quota_used NUMERIC(20,8) DEFAULT 0,status TEXT DEFAULT 'active',deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,
 usage_5h NUMERIC(20,8) DEFAULT 0,usage_1d NUMERIC(20,8) DEFAULT 0,usage_7d NUMERIC(20,8) DEFAULT 0,window_5h_start TIMESTAMPTZ,window_1d_start TIMESTAMPTZ,window_7d_start TIMESTAMPTZ);
 CREATE TABLE user_packages(id BIGINT PRIMARY KEY,user_id BIGINT NOT NULL);
 CREATE TABLE package_periods(id BIGINT PRIMARY KEY,package_id BIGINT,quota_usd NUMERIC(20,8),used_usd NUMERIC(20,8) DEFAULT 0);
 CREATE TABLE user_subscriptions(id BIGINT PRIMARY KEY,weekly_usage_usd NUMERIC(20,8) DEFAULT 0);
 INSERT INTO users VALUES(1,100),(2,100);INSERT INTO api_keys(id,user_id) VALUES(1,1);INSERT INTO user_packages VALUES(1,1),(2,2);
 INSERT INTO package_periods(id,package_id,quota_usd) VALUES(1,1,10),(2,1,10),(3,2,10);INSERT INTO user_subscriptions(id) VALUES(1);`)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile("241_package_usage_billing.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	return db
}

func TestPackagePostgresConcurrentReplayAndPeriodIsolation(t *testing.T) {
	db := packageBillingTestDB(t)
	repo := NewUsageBillingRepository(nil, db)
	ctx := context.Background()
	cmd := service.UsageBillingCommand{RequestID: "request", APIKeyID: 1, UserID: 1, PackageID: 1, PackagePeriodID: 1, PackageCost: 1.25, APIKeyQuotaCost: 1.25, APIKeyRateLimitCost: 1.25}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); copy := cmd; _, err := repo.Apply(ctx, &copy); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var used, next, balance, quota, legacy float64
	require.NoError(t, db.QueryRow(`SELECT used_usd FROM package_periods WHERE id=1`).Scan(&used))
	require.Equal(t, 1.25, used)
	require.NoError(t, db.QueryRow(`SELECT used_usd FROM package_periods WHERE id=2`).Scan(&next))
	require.Zero(t, next)
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, 100.0, balance)
	require.NoError(t, db.QueryRow(`SELECT quota_used FROM api_keys WHERE id=1`).Scan(&quota))
	require.Equal(t, used, quota)
	require.NoError(t, db.QueryRow(`SELECT weekly_usage_usd FROM user_subscriptions WHERE id=1`).Scan(&legacy))
	require.Zero(t, legacy)
	changed := cmd
	changed.PackagePeriodID = 2
	_, err := repo.Apply(ctx, &changed)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
	foreign := cmd
	foreign.RequestID = "foreign"
	foreign.PackageID = 2
	foreign.PackagePeriodID = 3
	_, err = repo.Apply(ctx, &foreign)
	require.Error(t, err)
	_, err = db.Exec(`UPDATE package_periods SET quota_usd=20 WHERE id=1`)
	require.NoError(t, err)
	copy := cmd
	result, err := repo.Apply(ctx, &copy)
	require.NoError(t, err)
	require.False(t, result.Applied)
}

func TestPackagePostgresFailedEffectsRetainRecoverableCommand(t *testing.T) {
	db := packageBillingTestDB(t)
	repo := NewUsageBillingRepository(nil, db)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE api_keys SET deleted_at=NOW() WHERE id=1`)
	require.NoError(t, err)
	cmd := &service.UsageBillingCommand{RequestID: "retry", APIKeyID: 1, UserID: 1, PackageID: 1, PackagePeriodID: 1, PackageCost: 2, APIKeyQuotaCost: 2}
	_, err = repo.Apply(ctx, cmd)
	require.Error(t, err)
	var used float64
	var applied bool
	require.NoError(t, db.QueryRow(`SELECT used_usd FROM package_periods WHERE id=1`).Scan(&used))
	require.Zero(t, used)
	require.NoError(t, db.QueryRow(`SELECT applied FROM package_usage_billing WHERE request_id='retry'`).Scan(&applied))
	require.False(t, applied)
	_, err = db.Exec(`UPDATE api_keys SET deleted_at=NULL WHERE id=1`)
	require.NoError(t, err)
	result, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NoError(t, db.QueryRow(`SELECT used_usd FROM package_periods WHERE id=1`).Scan(&used))
	require.Equal(t, 2.0, used)
}
