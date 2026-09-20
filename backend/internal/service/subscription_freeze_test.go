package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionFreezeClockPreservesAllCountdowns(t *testing.T) {
	base := timezone.StartOfDay(time.Date(2026, 9, 20, 12, 0, 0, 0, timezone.Location()))
	freezeAt := base.Add(20 * time.Hour)
	week := base.Add(-5 * 24 * time.Hour)
	month := base.Add(-20 * 24 * time.Hour)
	sub := &UserSubscription{StartsAt: base.Add(-22 * 24 * time.Hour), ExpiresAt: freezeAt.Add(5 * 24 * time.Hour), Status: SubscriptionStatusActive,
		DailyWindowStart: &base, WeeklyWindowStart: &week, MonthlyWindowStart: &month, DailyUsageUSD: 3, WeeklyUsageUSD: 20, MonthlyUsageUSD: 50, FrozenAt: &freezeAt}
	require.Equal(t, int64(5*86400), sub.RemainingSecondsAt(freezeAt.Add(3*24*time.Hour)))
	require.False(t, sub.IsActive())
	require.False(t, sub.NeedsDailyResetAt(freezeAt.Add(3*24*time.Hour)))
	dailyLeft := sub.DailyResetTime().Sub(freezeAt)
	weeklyLeft := sub.WeeklyResetTime().Sub(freezeAt)
	monthlyLeft := sub.MonthlyResetTime().Sub(freezeAt)
	require.Equal(t, 4*time.Hour, dailyLeft)
	delta := 3*24*time.Hour + 90*time.Minute
	thawAt := freezeAt.Add(delta)
	daily, weekly, monthly := base.Add(delta), week.Add(delta), month.Add(delta)
	sub.ExpiresAt = sub.ExpiresAt.Add(delta)
	sub.DailyWindowStart, sub.WeeklyWindowStart, sub.MonthlyWindowStart = &daily, &weekly, &monthly
	sub.FrozenDurationUS = delta.Microseconds()
	sub.FrozenAt = nil
	require.Equal(t, int64(5*86400), sub.RemainingSecondsAt(thawAt))
	require.Equal(t, dailyLeft, sub.DailyResetTime().Sub(thawAt))
	require.Equal(t, weeklyLeft, sub.WeeklyResetTime().Sub(thawAt))
	require.Equal(t, monthlyLeft, sub.MonthlyResetTime().Sub(thawAt))
	require.False(t, sub.NeedsDailyResetAt(thawAt.Add(3*time.Hour)), "crossing wall-clock midnight must not shorten the preserved four hours")
	require.True(t, sub.NeedsDailyResetAt(thawAt.Add(4*time.Hour)))
	require.Equal(t, 3.0, sub.DailyUsageUSD)
	dayCard := &UserSubscription{StartsAt: base, ExpiresAt: base.Add(24*time.Hour + delta), FrozenDurationUS: delta.Microseconds(), DailyWindowStart: &daily}
	require.True(t, dayCard.HasOneTimeDailyQuota())
	require.False(t, dayCard.NeedsDailyResetAt(thawAt.Add(10*time.Hour)))
}
