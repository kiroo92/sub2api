package service

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testPackagePlan() PackagePlan {
	return PackagePlan{GroupID: 1, Name: "Month", Price: 385, Currency: "CNY", ValidityDays: 30, BaseQuotaUSD: 1800, GroupBuyEnabled: true, GroupBuyHours: 48, ForSale: true, Tiers: []PackageTier{{3, 1980}, {5, 2100}, {10, 2400}}}
}

func TestPackagePeriodsAndValidation(t *testing.T) {
	plan := testPackagePlan()
	require.NoError(t, plan.Validate())
	start := time.Date(2026, 9, 10, 12, 30, 0, 0, time.UTC)
	periods := packagePeriods(plan, start, 1800)
	require.Len(t, periods, 4)
	for i, period := range periods {
		require.Equal(t, 450.0, period.QuotaUSD)
		require.Equal(t, start.Add(time.Duration(7*i)*24*time.Hour), period.StartsAt)
	}
	require.Equal(t, 9*24*time.Hour, periods[3].EndsAt.Sub(periods[3].StartsAt))
	require.True(t, start.Add(28*24*time.Hour).Before(periods[3].EndsAt))
	require.Equal(t, start.Add(30*24*time.Hour), periods[3].EndsAt)
	require.Equal(t, 1800.0, plan.quotaForMembers(2))
	require.Equal(t, 2100.0, plan.quotaForMembers(8))
	require.Equal(t, 2400.0, plan.quotaForMembers(10))
	require.Equal(t, 600.0, packagePeriods(plan, start, 2400)[3].QuotaUSD)
	week := plan
	week.ValidityDays = 7
	week.GroupBuyHours = 24
	require.NoError(t, week.Validate())
	require.Equal(t, 24, week.GroupBuyHours)
	require.Equal(t, 1800.0, packagePeriods(week, start, 1800)[0].QuotaUSD)
	for _, mutate := range []func(*PackagePlan){
		func(p *PackagePlan) { p.Price = math.NaN() }, func(p *PackagePlan) { p.BaseQuotaUSD = math.Inf(1) },
		func(p *PackagePlan) { p.BaseQuotaUSD = 0.00000001 }, func(p *PackagePlan) { p.ValidityDays = 28 },
		func(p *PackagePlan) { p.GroupBuyHours = 168 }, func(p *PackagePlan) { p.GroupBuyHours = -1 },
		func(p *PackagePlan) { p.GroupBuyHours = 0 },
		func(p *PackagePlan) { p.Tiers = []PackageTier{{3, 1980}, {3, 2100}} }, func(p *PackagePlan) { p.Tiers = []PackageTier{{3, 1700}} },
		func(p *PackagePlan) { p.ThemeColor = "#ff00aa" },
	} {
		p := testPackagePlan()
		mutate(&p)
		require.Error(t, p.Validate())
	}
}
