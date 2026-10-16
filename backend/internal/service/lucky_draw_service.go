package service

import (
	"context"
	"crypto/rand"
	"log/slog"
	"math"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

var (
	ErrLuckyDrawPaused          = infraerrors.Forbidden("LUCKY_DRAW_PAUSED", "幸运抽奖暂未开放")
	ErrLuckyDrawNoChances       = infraerrors.Forbidden("LUCKY_DRAW_NO_CHANCES", "抽奖次数不足")
	ErrLuckyDrawChanged         = infraerrors.Conflict("LUCKY_DRAW_CHANGED", "抽奖规则已更新，请刷新后重试")
	ErrLuckyDrawThresholdLocked = infraerrors.Conflict("LUCKY_DRAW_THRESHOLD_LOCKED", "已有抽奖记录，不能修改充值换次数门槛")
)

type LuckyDrawPrize struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Weight int     `json:"weight"`
}

type LuckyDrawConfig struct {
	Enabled         bool             `json:"enabled"`
	Version         int64            `json:"version"`
	RechargePerDraw float64          `json:"recharge_per_draw"`
	GuaranteeAfter  int              `json:"guarantee_after"`
	Prizes          []LuckyDrawPrize `json:"prizes"`
}

func (c LuckyDrawConfig) Validate() error {
	invalid := infraerrors.BadRequest("INVALID_LUCKY_DRAW_CONFIG", "请检查充值门槛、保底和奖池；金额最多两位小数，权重总和必须为 10000，且至少有一个余额奖品")
	money := func(v, max float64) bool {
		return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= max && math.Round(v*100)/100 == v
	}
	if !money(c.RechargePerDraw, 1000000) || c.RechargePerDraw < 0.01 || c.GuaranteeAfter < 0 || c.GuaranteeAfter > 1000 || len(c.Prizes) < 2 || len(c.Prizes) > 12 {
		return invalid
	}
	sum, positive := 0, false
	for _, p := range c.Prizes {
		if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > 80 || !money(p.Amount, 10000) || p.Weight < 1 || p.Weight > 10000 {
			return invalid
		}
		sum += p.Weight
		positive = positive || p.Amount > 0
	}
	if sum != 10000 || !positive {
		return invalid
	}
	return nil
}

type LuckyDrawRecord struct {
	ID            int64     `json:"id"`
	RequestID     string    `json:"request_id"`
	ConfigVersion int64     `json:"config_version"`
	PrizeIndex    int       `json:"prize_index"`
	PrizeName     string    `json:"prize_name"`
	PrizeAmount   float64   `json:"prize_amount"`
	Guaranteed    bool      `json:"guaranteed"`
	CreatedAt     time.Time `json:"created_at"`
}

type LuckyDrawSnapshot struct {
	Config         LuckyDrawConfig   `json:"config"`
	TotalRecharged float64           `json:"total_recharged"`
	Chances        int64             `json:"chances"`
	DrawsUsed      int64             `json:"draws_used"`
	Misses         int               `json:"misses"`
	History        []LuckyDrawRecord `json:"history"`
}

type LuckyDrawPicker func([]LuckyDrawPrize, bool) (int, error)
type LuckyDrawRepository interface {
	Snapshot(context.Context, int64) (*LuckyDrawSnapshot, error)
	Configure(context.Context, LuckyDrawConfig) error
	Draw(context.Context, int64, string, int64, LuckyDrawPicker) (*LuckyDrawRecord, error)
}

type LuckyDrawService struct {
	repo    LuckyDrawRepository
	billing *BillingCacheService
	auth    *APIKeyService
}

func NewLuckyDrawService(repo LuckyDrawRepository, billing *BillingCacheService, auth *APIKeyService) *LuckyDrawService {
	return &LuckyDrawService{repo: repo, billing: billing, auth: auth}
}
func (s *LuckyDrawService) Snapshot(ctx context.Context, userID int64) (*LuckyDrawSnapshot, error) {
	return s.repo.Snapshot(ctx, userID)
}
func (s *LuckyDrawService) Configure(ctx context.Context, c LuckyDrawConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return s.repo.Configure(ctx, c)
}
func (s *LuckyDrawService) Draw(ctx context.Context, userID int64, requestID string, version int64) (*LuckyDrawRecord, error) {
	id, err := uuid.Parse(requestID)
	if err != nil || id == uuid.Nil || version < 1 || userID < 1 {
		return nil, infraerrors.BadRequest("INVALID_LUCKY_DRAW_REQUEST", "无效的抽奖请求")
	}
	result, err := s.repo.Draw(ctx, userID, id.String(), version, pickLuckyDrawPrize)
	if err != nil {
		return nil, err
	}
	if result.PrizeAmount > 0 {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if s.billing != nil {
			if err := s.billing.InvalidateUserBalance(cacheCtx, userID); err != nil {
				slog.Warn("lucky draw balance cache invalidation failed", "user_id", userID, "error", err)
			}
		}
		if s.auth != nil {
			s.auth.InvalidateAuthCacheByUserID(cacheCtx, userID)
		}
	}
	return result, nil
}

func pickLuckyDrawPrize(prizes []LuckyDrawPrize, guaranteed bool) (int, error) {
	total := 0
	for _, p := range prizes {
		if !guaranteed || p.Amount > 0 {
			total += p.Weight
		}
	}
	if total <= 0 {
		return 0, infraerrors.BadRequest("INVALID_LUCKY_DRAW_CONFIG", "奖池没有可抽取的奖品")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(total)))
	if err != nil {
		return 0, err
	}
	target := int(n.Int64())
	for i, p := range prizes {
		if guaranteed && p.Amount == 0 {
			continue
		}
		target -= p.Weight
		if target < 0 {
			return i, nil
		}
	}
	return 0, infraerrors.BadRequest("INVALID_LUCKY_DRAW_CONFIG", "无效的奖池")
}
