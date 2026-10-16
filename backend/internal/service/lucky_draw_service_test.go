package service_test

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLuckyDrawConfigValidation(t *testing.T) {
	valid := func() service.LuckyDrawConfig {
		return service.LuckyDrawConfig{Version: 1, RechargePerDraw: 50, GuaranteeAfter: 10, Prizes: []service.LuckyDrawPrize{{Name: "余额", Amount: 0.5, Weight: 1}, {Name: "未中奖", Weight: 9999}}}
	}
	require.NoError(t, valid().Validate())
	for name, mutate := range map[string]func(*service.LuckyDrawConfig){
		"zero threshold":     func(c *service.LuckyDrawConfig) { c.RechargePerDraw = 0 },
		"nonfinite":          func(c *service.LuckyDrawConfig) { c.RechargePerDraw = math.Inf(1) },
		"subcent":            func(c *service.LuckyDrawConfig) { c.Prizes[0].Amount = 0.001 },
		"nan":                func(c *service.LuckyDrawConfig) { c.Prizes[0].Amount = math.NaN() },
		"all losses":         func(c *service.LuckyDrawConfig) { c.Prizes[0].Amount = 0 },
		"wrong weight total": func(c *service.LuckyDrawConfig) { c.Prizes[0].Weight = 2 },
		"zero weight":        func(c *service.LuckyDrawConfig) { c.Prizes[0].Weight = 0 },
		"empty name":         func(c *service.LuckyDrawConfig) { c.Prizes[0].Name = "  " },
		"negative guarantee": func(c *service.LuckyDrawConfig) { c.GuaranteeAfter = -1 },
		"too few prizes":     func(c *service.LuckyDrawConfig) { c.Prizes = c.Prizes[:1] },
	} {
		t.Run(name, func(t *testing.T) { c := valid(); mutate(&c); require.Error(t, c.Validate()) })
	}
}

func TestLuckyDrawGuaranteedPickerExcludesLoss(t *testing.T) {
	prizes := []service.LuckyDrawPrize{{Name: "未中奖", Weight: 9999}, {Name: "余额", Amount: 0.5, Weight: 1}}
	repo := &luckyPickerRepo{prizes: prizes}
	svc := service.NewLuckyDrawService(repo, nil, nil)
	for i := 0; i < 100; i++ {
		_, err := svc.Draw(context.Background(), 1, "d66d1ae0-9bea-4567-8ab7-9198e788cb6f", 1)
		require.NoError(t, err)
		require.Equal(t, 1, repo.index)
	}
	repo.prizes = prizes[:1]
	_, err := svc.Draw(context.Background(), 1, "d66d1ae0-9bea-4567-8ab7-9198e788cb6f", 1)
	require.Error(t, err)
}

type luckyPickerRepo struct {
	prizes []service.LuckyDrawPrize
	index  int
}

func (r *luckyPickerRepo) Snapshot(context.Context, int64) (*service.LuckyDrawSnapshot, error) {
	return nil, nil
}
func (r *luckyPickerRepo) Configure(context.Context, service.LuckyDrawConfig) error { return nil }
func (r *luckyPickerRepo) Draw(_ context.Context, _ int64, _ string, _ int64, pick service.LuckyDrawPicker) (*service.LuckyDrawRecord, error) {
	index, err := pick(r.prizes, true)
	r.index = index
	return &service.LuckyDrawRecord{}, err
}

func (r *luckyPickerRepo) AdminStats(context.Context, string, int) (*service.LuckyDrawAdminStats, error) {
	return &service.LuckyDrawAdminStats{}, nil
}

func (r *luckyPickerRepo) GrantChances(context.Context, int64, int64, string, int, string) (*service.LuckyDrawGrant, error) {
	return &service.LuckyDrawGrant{}, nil
}

func (r *luckyPickerRepo) DrawBatch(ctx context.Context, userID int64, requestID string, version int64, count int, pick service.LuckyDrawPicker) (*service.LuckyDrawBatch, error) {
	d, err := r.Draw(ctx, userID, requestID, version, pick)
	if err != nil {
		return nil, err
	}
	return &service.LuckyDrawBatch{Records: []service.LuckyDrawRecord{*d}}, nil
}
