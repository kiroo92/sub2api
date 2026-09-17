package service

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTeamLimitsAndNameValidation(t *testing.T) {
	for _, limit := range []float64{-1, math.NaN(), math.Inf(1), 1e11} {
		require.Error(t, (TeamLimits{Daily: limit}).Validate())
	}
	require.NoError(t, (TeamLimits{}).Validate())
	require.NoError(t, (TeamLimits{Daily: 1, Weekly: 10, Monthly: 100}).Validate())
	name, err := teamName("  Team  ")
	require.NoError(t, err)
	require.Equal(t, "Team", name)
	_, err = teamName(" \t ")
	require.Error(t, err)
}

func TestTeamBillingSnapshotExcludesCredentialsAndSurvivesLogMutation(t *testing.T) {
	user := &User{ID: 1, PasswordHash: "private-password"}
	key := &APIKey{ID: 2, Key: "private-key", Team: &TeamAttribution{MemberID: 3, RequestID: "admission"}}
	account := &Account{ID: 4, Credentials: map[string]any{"token": "private-token"}}
	log := &UsageLog{UserID: 1, APIKeyID: 2, AccountID: 4, RequestID: "bill", ActualCost: 0.5, User: user, APIKey: key, Account: account}
	cmd := buildUsageBillingCommand("bill", log, &postUsageBillingParams{User: user, APIKey: key, Account: account, Cost: &CostBreakdown{ActualCost: 0.5, TotalCost: 0.5}})
	require.NotNil(t, cmd.TeamUsageLog)
	log.ActualCost = 0
	require.Equal(t, 0.5, cmd.TeamUsageLog.ActualCost)
	data, err := json.Marshal(cmd)
	require.NoError(t, err)
	for _, secret := range []string{"private-password", "private-key", "private-token"} {
		require.NotContains(t, string(data), secret)
	}
	require.Same(t, user, log.User, "do not mutate caller log associations")
}
