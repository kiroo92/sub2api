package service

import (
	"context"
	"crypto/rand"
	"log/slog"
	"math"
	"math/big"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrLotteryPaused       = infraerrors.Forbidden("LOTTERY_PAUSED", "抽奖活动暂未开放")
	ErrLotteryRoundChanged = infraerrors.Conflict("LOTTERY_ROUND_CHANGED", "本期已结束，请刷新后参与新一期")
	ErrLotteryNotEligible  = infraerrors.Forbidden("LOTTERY_NOT_ELIGIBLE", "累计充值尚未达到本期参与门槛")
)

type LotteryConfig struct {
	TurnstileSiteKey          string `json:"turnstile_site_key"`
	TurnstileSecretKey        string `json:"-"`
	TurnstileSecretConfigured bool   `json:"turnstile_secret_configured"`

	Enabled           bool    `json:"enabled"`
	PrizeAmount       float64 `json:"prize_amount"`
	WinnerCount       int     `json:"winner_count"`
	ParticipantTarget int     `json:"participant_target"`
	MinRecharge       float64 `json:"min_recharge"`
}

func (c LotteryConfig) Validate() error {
	if len(c.TurnstileSiteKey) > 256 || len(c.TurnstileSecretKey) > 256 {
		return infraerrors.BadRequest("INVALID_LOTTERY_CONFIG", "Turnstile key is too long")
	}

	validMoney := func(v float64, max float64) bool {
		return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= max && math.Round(v*100)/100 == v
	}
	if !validMoney(c.PrizeAmount, 10000) || c.PrizeAmount < 0.01 || !validMoney(c.MinRecharge, 1000000) ||
		c.WinnerCount < 1 || c.WinnerCount > 100 || c.ParticipantTarget < 2 || c.ParticipantTarget > 10000 || c.WinnerCount > c.ParticipantTarget {
		return infraerrors.BadRequest("INVALID_LOTTERY_CONFIG", "请检查抽奖配置：奖金 0.01–10000，名额 1–100，开奖人数 2–10000 且不少于名额，充值门槛 0–1000000；金额最多两位小数")
	}
	return nil
}

type LotteryRound struct {
	ID                int64      `json:"id"`
	PrizeAmount       float64    `json:"prize_amount"`
	WinnerCount       int        `json:"winner_count"`
	ParticipantTarget int        `json:"participant_target"`
	MinRecharge       float64    `json:"min_recharge"`
	ParticipantCount  int        `json:"participant_count"`
	WinnersDrawn      int        `json:"winners_drawn"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	DrawnAt           *time.Time `json:"drawn_at"`
}

type LotteryWin struct {
	RoundID     int64     `json:"round_id"`
	UserLabel   string    `json:"user_label"`
	PrizeAmount float64   `json:"prize_amount"`
	AwardedAt   time.Time `json:"awarded_at"`
	Email       string    `json:"-"`
}

type LotterySnapshot struct {
	Config         LotteryConfig  `json:"config"`
	Current        *LotteryRound  `json:"current"`
	Joined         bool           `json:"joined"`
	Eligible       bool           `json:"eligible"`
	TotalRecharged float64        `json:"total_recharged"`
	RecentWinners  []LotteryWin   `json:"recent_winners"`
	MyWins         []LotteryWin   `json:"my_wins"`
	RecentRounds   []LotteryRound `json:"recent_rounds"`
}

type LotteryJoinResult struct {
	RoundID       int64   `json:"round_id"`
	AlreadyJoined bool    `json:"already_joined"`
	Drawn         bool    `json:"drawn"`
	WinnerIDs     []int64 `json:"-"`
}

type LotteryWinnerPicker func([]int64, int) ([]int64, error)

type LotteryRepository interface {
	Snapshot(context.Context, int64) (*LotterySnapshot, error)
	Configure(context.Context, LotteryConfig) error
	Join(context.Context, int64, int64, LotteryWinnerPicker) (*LotteryJoinResult, error)
}

type LotteryService struct {
	repo    LotteryRepository
	billing *BillingCacheService
	auth    *APIKeyService
}

func NewLotteryService(repo LotteryRepository, billing *BillingCacheService, auth *APIKeyService) *LotteryService {
	return &LotteryService{repo: repo, billing: billing, auth: auth}
}

func (s *LotteryService) Snapshot(ctx context.Context, userID int64) (*LotterySnapshot, error) {
	snapshot, err := s.repo.Snapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	snapshot.Config.TurnstileSecretConfigured = snapshot.Config.TurnstileSecretKey != ""
	snapshot.Config.TurnstileSecretKey = ""
	for i := range snapshot.RecentWinners {
		snapshot.RecentWinners[i].UserLabel = maskLotteryEmail(snapshot.RecentWinners[i].Email)
		snapshot.RecentWinners[i].Email = ""
	}
	for i := range snapshot.MyWins {
		snapshot.MyWins[i].UserLabel = ""
		snapshot.MyWins[i].Email = ""
	}
	return snapshot, nil
}

func (s *LotteryService) Configure(ctx context.Context, config LotteryConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	return s.repo.Configure(ctx, config)
}

func (s *LotteryService) Join(ctx context.Context, userID, roundID int64) (*LotteryJoinResult, error) {
	if userID <= 0 || roundID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_LOTTERY_ROUND", "请选择有效的抽奖期数")
	}
	result, err := s.repo.Join(ctx, userID, roundID, pickLotteryWinners)
	if err != nil {
		return nil, err
	}
	// Credit has already committed. Cache refresh failure must never retry payment.
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	for _, id := range result.WinnerIDs {
		if s.billing != nil {
			if err := s.billing.InvalidateUserBalance(cacheCtx, id); err != nil {
				slog.Warn("lottery balance cache invalidation failed", "user_id", id, "error", err)
			}
		}
		if s.auth != nil {
			s.auth.InvalidateAuthCacheByUserID(cacheCtx, id)
		}
	}
	return result, nil
}

// Partial Fisher-Yates using cryptographic randomness, sampling without replacement.
func pickLotteryWinners(ids []int64, count int) ([]int64, error) {
	if count < 1 || count > len(ids) {
		return nil, infraerrors.BadRequest("INVALID_LOTTERY_DRAW", "抽奖人数不足")
	}
	pool := append([]int64(nil), ids...)
	for i := 0; i < count; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pool)-i)))
		if err != nil {
			return nil, err
		}
		j := i + int(n.Int64())
		pool[i], pool[j] = pool[j], pool[i]
	}
	return pool[:count], nil
}

func maskLotteryEmail(email string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" {
		return "***"
	}
	chars := []rune(local)
	if len(chars) <= 2 {
		return string(chars[:1]) + "***@" + domain
	}
	return string(chars[:1]) + "***" + string(chars[len(chars)-1:]) + "@" + domain
}
