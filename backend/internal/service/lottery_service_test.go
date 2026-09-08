package service

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestLotteryWinnerSelection(t *testing.T) {
	ids := make([]int64, 60)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	for i := 0; i < 20; i++ {
		winners, err := pickLotteryWinners(ids, 6)
		require.NoError(t, err)
		require.Len(t, winners, 6)
		seen := map[int64]bool{}
		for _, id := range winners {
			require.False(t, seen[id])
			seen[id] = true
			require.Contains(t, ids, id)
		}
	}
	require.Equal(t, int64(1), ids[0], "input ordering is not mutated")
	_, err := pickLotteryWinners(ids, 61)
	require.Error(t, err)
	_, err = pickLotteryWinners(nil, 1)
	require.Error(t, err)
	winners, err := pickLotteryWinners(ids, 60)
	require.NoError(t, err)
	require.ElementsMatch(t, ids, winners)
}
func TestLotteryConfigValidation(t *testing.T) {
	valid := LotteryConfig{PrizeAmount: 5, WinnerCount: 6, ParticipantTarget: 60, MinRecharge: 50}
	require.NoError(t, valid.Validate())
	for _, mutate := range []func(*LotteryConfig){
		func(c *LotteryConfig) { c.PrizeAmount = math.NaN() }, func(c *LotteryConfig) { c.PrizeAmount = math.Inf(1) }, func(c *LotteryConfig) { c.PrizeAmount = 0 }, func(c *LotteryConfig) { c.PrizeAmount = 0.001 }, func(c *LotteryConfig) { c.PrizeAmount = 0.000000001 }, func(c *LotteryConfig) { c.MinRecharge = -1 }, func(c *LotteryConfig) { c.ParticipantTarget = 5 }, func(c *LotteryConfig) { c.WinnerCount = 101 }, func(c *LotteryConfig) { c.ParticipantTarget = 10001 },
	} {
		c := valid
		mutate(&c)
		require.Error(t, c.Validate())
	}
	valid.MinRecharge = 0
	valid.PrizeAmount = 0.01
	require.NoError(t, valid.Validate())
}
func TestLotteryMaskEmail(t *testing.T) {
	for input, want := range map[string]string{"person@example.com": "p***n@example.com", "a@example.com": "a***@example.com", "ab@example.com": "a***@example.com", "用户名称@example.com": "用***称@example.com", "invalid": "***", "@example.com": "***"} {
		require.Equal(t, want, maskLotteryEmail(input))
	}
}
