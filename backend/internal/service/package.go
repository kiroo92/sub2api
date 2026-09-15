package service

import (
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const OrderTypePackage = "package"

var packageThemeColors = map[string]struct{}{
	"violet": {}, "emerald": {}, "blue": {}, "orange": {},
	"rose": {}, "cyan": {}, "amber": {}, "indigo": {},
}

type PackageTier struct {
	Members  int     `json:"members"`
	QuotaUSD float64 `json:"quota_usd"`
}

type PackagePlan struct {
	ID              int64         `json:"id"`
	GroupID         int64         `json:"group_id"`
	GroupName       string        `json:"group_name"`
	GroupPlatform   string        `json:"group_platform"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	Price           float64       `json:"price"`
	Currency        string        `json:"currency"`
	ValidityDays    int           `json:"validity_days"`
	BaseQuotaUSD    float64       `json:"base_quota_usd"`
	GroupBuyEnabled bool          `json:"group_buy_enabled"`
	GroupBuyHours   int           `json:"group_buy_hours"`
	Tiers           []PackageTier `json:"tiers"`
	ThemeColor      string        `json:"theme_color,omitempty"`
	ForSale         bool          `json:"for_sale"`
	SortOrder       int           `json:"sort_order"`
}

func (p *PackagePlan) Validate() error {
	bad := infraerrors.BadRequest("INVALID_PACKAGE_PLAN", "invalid package price, quota, periods or group-buy tiers")
	p.Name = strings.TrimSpace(p.Name)
	var err error
	p.Currency, err = payment.NormalizePaymentCurrency(p.Currency)
	if err != nil {
		return bad
	}
	if p.GroupID <= 0 || p.Name == "" || len(p.Name) > 100 || len(p.Description) > 4000 || (p.ValidityDays != 7 && p.ValidityDays != 30) {
		return bad
	}
	if p.ThemeColor != "" {
		if _, ok := packageThemeColors[p.ThemeColor]; !ok {
			return bad
		}
	}
	if !packagePositiveMoney(p.Price) || validateCreateOrderAmountCurrency(p.Price, p.Currency) != nil {
		return bad
	}
	count := p.periodCount()
	validQuota := func(value float64) bool {
		if !packagePositiveMoney(value) {
			return false
		}
		d := decimal.NewFromFloat(value).Div(decimal.NewFromInt(int64(count)))
		return d.Equal(d.Round(8))
	}
	if !validQuota(p.BaseQuotaUSD) {
		return bad
	}
	if p.GroupBuyHours <= 0 || p.GroupBuyHours >= 168 || len(p.Tiers) > 20 || (p.GroupBuyEnabled && len(p.Tiers) == 0) {
		return bad
	}
	lastMembers, lastQuota := 1, p.BaseQuotaUSD
	for _, tier := range p.Tiers {
		if tier.Members <= lastMembers || tier.Members > 10000 || !validQuota(tier.QuotaUSD) || tier.QuotaUSD < lastQuota {
			return bad
		}
		lastMembers, lastQuota = tier.Members, tier.QuotaUSD
	}
	if p.Tiers == nil {
		p.Tiers = []PackageTier{}
	}
	return nil
}

func packagePositiveMoney(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 && v <= 1e9 && decimal.NewFromFloat(v).Equal(decimal.NewFromFloat(v).Round(8))
}
func (p PackagePlan) periodCount() int {
	if p.ValidityDays == 30 {
		return 4
	}
	return 1
}
func (p PackagePlan) quotaForMembers(members int) float64 {
	quota := p.BaseQuotaUSD
	for _, tier := range p.Tiers {
		if members >= tier.Members {
			quota = tier.QuotaUSD
		}
	}
	return quota
}
func (p PackagePlan) targetMembers() int {
	if len(p.Tiers) == 0 {
		return 1
	}
	return p.Tiers[len(p.Tiers)-1].Members
}

type PackagePeriod struct {
	ID        int64     `json:"id"`
	PackageID int64     `json:"package_id"`
	Index     int       `json:"period_index"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	QuotaUSD  float64   `json:"quota_usd"`
	UsedUSD   float64   `json:"used_usd"`
}

func packagePeriods(plan PackagePlan, started time.Time, quota float64) []PackagePeriod {
	periods := make([]PackagePeriod, plan.periodCount())
	allocation := decimal.NewFromFloat(quota).Div(decimal.NewFromInt(int64(len(periods)))).InexactFloat64()
	for i := range periods {
		end := (i + 1) * 7
		if i == len(periods)-1 {
			end = plan.ValidityDays
		}
		periods[i] = PackagePeriod{Index: i + 1, StartsAt: started.Add(time.Duration(i*7) * 24 * time.Hour), EndsAt: started.Add(time.Duration(end) * 24 * time.Hour), QuotaUSD: allocation}
	}
	return periods
}

type UserPackage struct {
	ID            int64           `json:"id"`
	UserID        int64           `json:"user_id"`
	GroupID       int64           `json:"group_id"`
	OrderID       int64           `json:"order_id"`
	Plan          PackagePlan     `json:"plan"`
	GroupBuyID    *int64          `json:"group_buy_id"`
	Status        string          `json:"status"`
	StartsAt      time.Time       `json:"starts_at"`
	ExpiresAt     time.Time       `json:"expires_at"`
	SortOrder     int64           `json:"sort_order"`
	Periods       []PackagePeriod `json:"periods"`
	CurrentPeriod *PackagePeriod  `json:"current_period"`
}

type PackageGroupBuy struct {
	ID            int64       `json:"id"`
	CreatorID     int64       `json:"-"`
	Plan          PackagePlan `json:"plan"`
	Status        string      `json:"status"`
	PaidCount     int         `json:"paid_count"`
	TargetMembers int         `json:"target_members"`
	StartsAt      *time.Time  `json:"starts_at"`
	EndsAt        *time.Time  `json:"ends_at"`
	SettledAt     *time.Time  `json:"settled_at"`
	FinalMembers  *int        `json:"final_members"`
	FinalQuotaUSD *float64    `json:"final_quota_usd"`
	Joined        bool        `json:"joined"`
	OrderID       *int64      `json:"order_id"`
}
