package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Uses a disposable database and an isolated schema, never application data.
func TestUserDashboardCostsPostgres(t *testing.T) {
	dsn := os.Getenv("DASHBOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set DASHBOARD_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("dashboard_test_%d", time.Now().UnixNano())
	_, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		_, err := base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
		require.NoError(t, base.Close())
	})
	_, err = db.ExecContext(ctx, `
		CREATE TABLE api_keys (user_id bigint, status text, deleted_at timestamptz);
		CREATE TABLE groups (id bigint, platform text);
		CREATE TABLE accounts (id bigint, platform text);
		CREATE TABLE usage_logs (
			user_id bigint, api_key_id bigint, group_id bigint, account_id bigint,
			billing_type smallint, actual_cost numeric, total_cost numeric,
			input_tokens bigint, output_tokens bigint, cache_creation_tokens bigint,
			cache_read_tokens bigint, duration_ms int, created_at timestamptz
		);
		INSERT INTO api_keys VALUES (1, 'active', NULL);
		INSERT INTO groups VALUES (1, 'openai');
	`)
	require.NoError(t, err)
	today := timezone.Today()
	for _, row := range []struct {
		user, billing int
		cost          float64
		at            time.Time
	}{
		{1, 0, 0.5, today}, // Include the exact local day boundary.
		{1, 1, 1.25, today.Add(time.Second)},
		{1, 1, 0, today.Add(2 * time.Second)}, // Free requests still count.
		{1, 0, 9, today.Add(-time.Microsecond)},
		{2, 1, 999, today}, // Another user's spend must not leak.
	} {
		_, err = db.ExecContext(ctx, `INSERT INTO usage_logs VALUES ($1, $1, 1, 1, $2, $3::numeric, $3::numeric * 2, 10, 20, 30, 40, 100, $4)`, row.user, row.billing, row.cost, row.at)
		require.NoError(t, err)
	}
	repo := &usageLogRepository{sql: db}
	stats, err := repo.GetUserDashboardStats(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.TodayRequests)
	require.Equal(t, int64(4), stats.TotalRequests)
	require.Equal(t, int64(300), stats.TodayTokens)
	require.Equal(t, 1.75, stats.TodayActualCost)
	require.Equal(t, 3.5, stats.TodayCost)
	require.NotNil(t, stats.TodaySubscriptionCost)
	require.NotNil(t, stats.TodayBalanceCost)
	require.Equal(t, 1.25, *stats.TodaySubscriptionCost)
	require.Equal(t, 0.5, *stats.TodayBalanceCost)
	data, err := json.Marshal(stats)
	require.NoError(t, err)
	require.Contains(t, string(data), `"today_subscription_cost":1.25`)

	empty, err := repo.GetUserDashboardStats(ctx, 3)
	require.NoError(t, err)
	require.NotNil(t, empty.TodayBalanceCost)
	require.NotNil(t, empty.TodaySubscriptionCost)
	require.Zero(t, *empty.TodayBalanceCost)
	require.Zero(t, *empty.TodaySubscriptionCost)

	keyStats, err := repo.GetAPIKeyDashboardStats(ctx, 1)
	require.NoError(t, err)
	data, err = json.Marshal(keyStats)
	require.NoError(t, err)
	require.NotContains(t, string(data), "today_subscription_cost")
	require.NotContains(t, string(data), "today_balance_cost")
}
