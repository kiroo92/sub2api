package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// PackageService owns independent commerce SQL. The Ent transaction client lets
// checkout and its package sidecar commit together without changing old schemas.
type PackageService struct {
	client           *dbent.Client
	groups           GroupRepository
	settlementOnce   sync.Once
	settlementCancel context.CancelFunc
	settlementWG     sync.WaitGroup
}

func NewPackageService(client *dbent.Client, groups GroupRepository) *PackageService {
	return &PackageService{client: client, groups: groups}
}

func packageScan(ctx context.Context, client *dbent.Client, query string, args []any, dest ...any) error {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest...)
}

func (s *PackageService) ListPlans(ctx context.Context, all bool) ([]PackagePlan, error) {
	rows, err := s.client.QueryContext(ctx, `SELECT id,group_id,terms,for_sale,sort_order FROM package_plans WHERE ($1 OR for_sale) ORDER BY sort_order,id`, all)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	plans := []PackagePlan{}
	for rows.Next() {
		var p PackagePlan
		var data []byte
		if err = rows.Scan(&p.ID, &p.GroupID, &data, &p.ForSale, &p.SortOrder); err != nil {
			return nil, err
		}
		id, gid, sale, order := p.ID, p.GroupID, p.ForSale, p.SortOrder
		if err = json.Unmarshal(data, &p); err != nil {
			return nil, err
		}
		p.ID, p.GroupID, p.ForSale, p.SortOrder = id, gid, sale, order
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (s *PackageService) GetPlan(ctx context.Context, id int64) (*PackagePlan, error) {
	var p PackagePlan
	var data []byte
	err := packageScan(ctx, s.client, `SELECT id,group_id,terms,for_sale,sort_order FROM package_plans WHERE id=$1`, []any{id}, &p.ID, &p.GroupID, &data, &p.ForSale, &p.SortOrder)
	if err == sql.ErrNoRows {
		return nil, infraerrors.NotFound("PACKAGE_PLAN_NOT_FOUND", "package plan not found")
	}
	if err != nil {
		return nil, err
	}
	id, gid, sale, order := p.ID, p.GroupID, p.ForSale, p.SortOrder
	if err = json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	p.ID, p.GroupID, p.ForSale, p.SortOrder = id, gid, sale, order
	return &p, nil
}

func (s *PackageService) SavePlan(ctx context.Context, p PackagePlan) (*PackagePlan, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	g, err := s.groups.GetByID(ctx, p.GroupID)
	if err != nil || g == nil || g.Status != StatusActive {
		return nil, infraerrors.BadRequest("PACKAGE_GROUP_UNAVAILABLE", "package group is unavailable")
	}
	p.GroupName, p.GroupPlatform = g.Name, g.Platform
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if p.ID == 0 {
		err = packageScan(ctx, s.client, `INSERT INTO package_plans(group_id,terms,for_sale,sort_order) VALUES($1,$2,$3,$4) RETURNING id`, []any{p.GroupID, string(data), p.ForSale, p.SortOrder}, &p.ID)
	} else {
		err = packageScan(ctx, s.client, `UPDATE package_plans SET group_id=$2,terms=$3,for_sale=$4,sort_order=$5,updated_at=NOW() WHERE id=$1 RETURNING id`, []any{p.ID, p.GroupID, string(data), p.ForSale, p.SortOrder}, &p.ID)
	}
	if err == sql.ErrNoRows {
		return nil, infraerrors.NotFound("PACKAGE_PLAN_NOT_FOUND", "package plan not found")
	}
	return &p, err
}

func (s *PackageService) StopSale(ctx context.Context, id int64) error {
	_, err := s.client.ExecContext(ctx, `UPDATE package_plans SET for_sale=FALSE,updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (s *PackageService) ListOwned(ctx context.Context, userID int64) ([]UserPackage, error) {
	rows, err := s.client.QueryContext(ctx, `SELECT p.id,p.user_id,p.group_id,p.order_id,p.plan_snapshot,p.group_buy_id,p.status,p.starts_at,p.expires_at,p.sort_order,
 r.id,r.period_index,r.starts_at,r.ends_at,r.quota_usd,r.used_usd
 FROM user_packages p JOIN package_periods r ON r.package_id=p.id WHERE p.user_id=$1 ORDER BY p.sort_order,p.id,r.period_index`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []UserPackage{}
	now := time.Now()
	for rows.Next() {
		var p UserPackage
		var r PackagePeriod
		var data []byte
		if err = rows.Scan(&p.ID, &p.UserID, &p.GroupID, &p.OrderID, &data, &p.GroupBuyID, &p.Status, &p.StartsAt, &p.ExpiresAt, &p.SortOrder, &r.ID, &r.Index, &r.StartsAt, &r.EndsAt, &r.QuotaUSD, &r.UsedUSD); err != nil {
			return nil, err
		}
		r.PackageID = p.ID
		if len(result) == 0 || result[len(result)-1].ID != p.ID {
			if err = json.Unmarshal(data, &p.Plan); err != nil {
				return nil, err
			}
			p.Periods = []PackagePeriod{}
			result = append(result, p)
		}
		last := &result[len(result)-1]
		last.Periods = append(last.Periods, r)
		if !now.Before(r.StartsAt) && now.Before(r.EndsAt) {
			rr := r
			last.CurrentPeriod = &rr
		}
	}
	return result, rows.Err()
}

func (s *PackageService) Reorder(ctx context.Context, userID int64, ids []int64) error {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	c := tx.Client()
	var locked int64
	if err = packageScan(ctx, c, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, []any{userID}, &locked); err != nil {
		return err
	}
	rows, err := c.QueryContext(ctx, `SELECT id FROM user_packages WHERE user_id=$1 AND status='active' AND expires_at>NOW()`, userID)
	if err != nil {
		return err
	}
	owned := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		owned[id] = true
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if len(ids) != len(owned) {
		return infraerrors.Conflict("PACKAGE_ORDER_CHANGED", "refresh packages before changing order")
	}
	for _, id := range ids {
		if !owned[id] {
			return infraerrors.BadRequest("INVALID_PACKAGE_ORDER", "package order contains duplicate or unowned IDs")
		}
		delete(owned, id)
	}
	for i, id := range ids {
		if _, err = c.ExecContext(ctx, `UPDATE user_packages SET sort_order=$3 WHERE id=$1 AND user_id=$2`, id, userID, i+1); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PackageService) CreateGroup(ctx context.Context, userID, planID int64) (*PackageGroupBuy, error) {
	p, err := s.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}
	if !p.ForSale || !p.GroupBuyEnabled {
		return nil, infraerrors.BadRequest("PACKAGE_GROUP_BUY_UNAVAILABLE", "group buying is unavailable for this plan")
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var id int64
	err = packageScan(ctx, s.client, `INSERT INTO package_group_buys(creator_id,plan_id,plan_snapshot) VALUES($1,$2,$3)
 ON CONFLICT(creator_id,plan_id) WHERE status='draft' DO UPDATE SET creator_id=EXCLUDED.creator_id RETURNING id`, []any{userID, planID, string(data)}, &id)
	if err != nil {
		return nil, err
	}
	return s.GetGroup(ctx, userID, id)
}

const packageGroupColumns = `g.id,g.creator_id,g.plan_snapshot,g.status,g.paid_count,g.starts_at,g.ends_at,g.settled_at,g.final_members,g.final_quota_usd`

func scanPackageGroup(rows *sql.Rows, g *PackageGroupBuy, personal bool) error {
	var data []byte
	dest := []any{&g.ID, &g.CreatorID, &data, &g.Status, &g.PaidCount, &g.StartsAt, &g.EndsAt, &g.SettledAt, &g.FinalMembers, &g.FinalQuotaUSD}
	if personal {
		dest = append(dest, &g.Joined, &g.OrderID)
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &g.Plan); err != nil {
		return err
	}
	g.TargetMembers = g.Plan.targetMembers()
	return nil
}
func (s *PackageService) ListGroups(ctx context.Context, userID int64) ([]PackageGroupBuy, error) {
	rows, err := s.client.QueryContext(ctx, `SELECT `+packageGroupColumns+`,(o.paid_at IS NOT NULL),o.order_id FROM package_group_buys g LEFT JOIN package_orders o ON o.group_buy_id=g.id AND o.user_id=$1 WHERE g.status='open' ORDER BY g.ends_at,g.id LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []PackageGroupBuy{}
	for rows.Next() {
		var g PackageGroupBuy
		if err = scanPackageGroup(rows, &g, true); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
func (s *PackageService) GetGroup(ctx context.Context, userID, id int64) (*PackageGroupBuy, error) {
	rows, err := s.client.QueryContext(ctx, `SELECT `+packageGroupColumns+`,(o.paid_at IS NOT NULL),o.order_id FROM package_group_buys g LEFT JOIN package_orders o ON o.group_buy_id=g.id AND o.user_id=$1 WHERE g.id=$2 AND (g.status<>'draft' OR g.creator_id=$1)`, userID, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return nil, err
		}
		return nil, infraerrors.NotFound("PACKAGE_GROUP_NOT_FOUND", "package group buy not found")
	}
	var g PackageGroupBuy
	err = scanPackageGroup(rows, &g, true)
	return &g, err
}

func packageLockedGroup(ctx context.Context, c *dbent.Client, id int64) (*PackageGroupBuy, error) {
	rows, err := c.QueryContext(ctx, `SELECT `+packageGroupColumns+` FROM package_group_buys g WHERE g.id=$1 FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	var g PackageGroupBuy
	err = scanPackageGroup(rows, &g, false)
	return &g, err
}

// checkoutPlan runs before provider selection; attachment below validates the
// locked group again so a concurrent close never creates a new payable member.
func (s *PackageService) checkoutPlan(ctx context.Context, req CreateOrderRequest) (*PackagePlan, error) {
	if req.PackagePlanID <= 0 || req.GroupBuyID < 0 || req.PlanID != 0 {
		return nil, infraerrors.BadRequest("INVALID_PACKAGE_CHECKOUT", "use a valid package plan and optional group buy, not a legacy subscription plan")
	}
	var p *PackagePlan
	var err error
	if req.GroupBuyID > 0 {
		g, e := s.GetGroup(ctx, req.UserID, req.GroupBuyID)
		if e != nil {
			return nil, e
		}
		p = &g.Plan
		if req.PackagePlanID != p.ID {
			return nil, infraerrors.BadRequest("PACKAGE_PLAN_MISMATCH", "group and plan do not match")
		}
	} else {
		p, err = s.GetPlan(ctx, req.PackagePlanID)
		if err != nil {
			return nil, err
		}
	}
	current, err := s.GetPlan(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if !current.ForSale {
		return nil, infraerrors.Forbidden("PACKAGE_NOT_FOR_SALE", "package is not for sale")
	}
	g, err := s.groups.GetByID(ctx, p.GroupID)
	if err != nil || g == nil || g.Status != StatusActive {
		return nil, infraerrors.Forbidden("PACKAGE_GROUP_UNAVAILABLE", "package group is unavailable")
	}
	return p, nil
}

func (s *PackageService) lockCheckout(ctx context.Context, c *dbent.Client, req CreateOrderRequest) error {
	if req.GroupBuyID == 0 {
		return nil
	}
	g, err := packageLockedGroup(ctx, c, req.GroupBuyID)
	if err != nil {
		return err
	}
	if g.Status == "settled" || (g.EndsAt != nil && !time.Now().Before(*g.EndsAt)) || (g.Status == "draft" && g.CreatorID != req.UserID) {
		return infraerrors.Conflict("PACKAGE_GROUP_CLOSED", "group buying has closed")
	}
	var orderID int64
	err = packageScan(ctx, c, `SELECT order_id FROM package_orders WHERE group_buy_id=$1 AND user_id=$2`, []any{req.GroupBuyID, req.UserID}, &orderID)
	if err == nil {
		return infraerrors.Conflict("PACKAGE_ALREADY_JOINED", "resume the existing order; each user may participate only once").WithMetadata(map[string]string{"order_id": strconv.FormatInt(orderID, 10)})
	}
	if err != sql.ErrNoRows {
		return err
	}
	return nil
}

func (s *PackageService) attachOrder(ctx context.Context, c *dbent.Client, req CreateOrderRequest, orderID int64) error {
	data, err := json.Marshal(req.packagePlan)
	if err != nil {
		return err
	}
	var gid any
	if req.GroupBuyID > 0 {
		gid = req.GroupBuyID
	}
	_, err = c.ExecContext(ctx, `INSERT INTO package_orders(order_id,user_id,plan_id,plan_snapshot,group_buy_id) VALUES($1,$2,$3,$4,$5)`, orderID, req.UserID, req.PackagePlanID, string(data), gid)
	return err
}

// fulfill issues one holding and paid membership atomically, including a frozen
// group reward for verified late notifications. It never changes old holdings.
func (s *PackageService) fulfill(ctx context.Context, order *dbent.PaymentOrder) error {
	if order.PaidAt == nil {
		return fmt.Errorf("package order %d is not paid", order.ID)
	}
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	c := tx.Client()
	var data []byte
	var userID int64
	var groupID *int64
	if err = packageScan(ctx, c, `SELECT user_id,plan_snapshot,group_buy_id FROM package_orders WHERE order_id=$1`, []any{order.ID}, &userID, &data, &groupID); err != nil {
		return err
	}
	if userID != order.UserID {
		return fmt.Errorf("package owner mismatch")
	}
	var plan PackagePlan
	if err = json.Unmarshal(data, &plan); err != nil {
		return err
	}
	var group *PackageGroupBuy
	if groupID != nil {
		group, err = packageLockedGroup(ctx, c, *groupID)
		if err != nil {
			return err
		}
	}
	// Serialize append ordering with user reorder operations and other purchases.
	var locked int64
	if err = packageScan(ctx, c, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, []any{userID}, &locked); err != nil {
		return err
	}
	var packageID int64
	err = packageScan(ctx, c, `SELECT id FROM user_packages WHERE order_id=$1`, []any{order.ID}, &packageID)
	if err == nil {
		return tx.Commit()
	}
	if err != sql.ErrNoRows {
		return err
	}
	quota := plan.BaseQuotaUSD
	if group != nil && group.FinalQuotaUSD != nil {
		quota = *group.FinalQuotaUSD
	}
	started := *order.PaidAt
	expires := started.Add(time.Duration(plan.ValidityDays) * 24 * time.Hour)
	err = packageScan(ctx, c, `INSERT INTO user_packages(user_id,group_id,order_id,plan_snapshot,group_buy_id,starts_at,expires_at,sort_order)
 SELECT $1,$2,$3,$4,$5,$6,$7,COALESCE(MAX(sort_order),0)+1 FROM user_packages WHERE user_id=$1 RETURNING id`, []any{userID, plan.GroupID, order.ID, string(data), groupID, started, expires}, &packageID)
	if err != nil {
		return err
	}
	for _, p := range packagePeriods(plan, started, quota) {
		if _, err = c.ExecContext(ctx, `INSERT INTO package_periods(package_id,period_index,starts_at,ends_at,quota_usd) VALUES($1,$2,$3,$4,$5)`, packageID, p.Index, p.StartsAt, p.EndsAt, p.QuotaUSD); err != nil {
			return err
		}
	}
	if _, err = c.ExecContext(ctx, `UPDATE package_orders SET paid_at=$2 WHERE order_id=$1`, order.ID, started); err != nil {
		return err
	}
	if group != nil && group.Status != "settled" {
		// A checkout prepared before deadline can still be paid afterwards. It
		// receives the frozen reward, but cannot increase the deadline's count.
		if group.EndsAt == nil || started.Before(*group.EndsAt) {
			group.PaidCount++
		}
		if group.Status == "draft" {
			end := started.Add(time.Duration(plan.GroupBuyHours) * time.Hour)
			group.Status = "open"
			group.StartsAt = &started
			group.EndsAt = &end
		}
		if _, err = c.ExecContext(ctx, `UPDATE package_group_buys SET status=$2,paid_count=$3,starts_at=$4,ends_at=$5 WHERE id=$1`, group.ID, group.Status, group.PaidCount, group.StartsAt, group.EndsAt); err != nil {
			return err
		}
		if group.PaidCount >= plan.targetMembers() {
			if err = settlePackageGroup(ctx, c, group); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func settlePackageGroup(ctx context.Context, c *dbent.Client, g *PackageGroupBuy) error {
	if g.Status == "settled" {
		return nil
	}
	quota := g.Plan.quotaForMembers(g.PaidCount)
	if _, err := c.ExecContext(ctx, `UPDATE package_group_buys SET status='settled',settled_at=NOW(),final_members=paid_count,final_quota_usd=$2 WHERE id=$1`, g.ID, quota); err != nil {
		return err
	}
	// Set an absolute target. Concurrent actual use is never overwritten.
	_, err := c.ExecContext(ctx, `UPDATE package_periods SET quota_usd=$2 WHERE package_id IN(SELECT id FROM user_packages WHERE group_buy_id=$1)`, g.ID, decimalAllocation(quota, g.Plan.periodCount()))
	return err
}

func decimalAllocation(quota float64, count int) float64 {
	return decimal.NewFromFloat(quota).Div(decimal.NewFromInt(int64(count))).InexactFloat64()
}
