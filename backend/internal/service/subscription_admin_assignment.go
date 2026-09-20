package service

import (
	"context"
	"encoding/json"
	"fmt"
)

// An explicit administrator operation creates a fresh entitlement. Its marker
// is written with the row, so retrying a partially completed batch cannot reissue it.
func (s *SubscriptionService) assignIndependentSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, error) {
	if input == nil {
		return nil, ErrSubscriptionNilInput
	}
	copy := *input
	copy.AdminAssignmentKey = HashIdempotencyKey(fmt.Sprintf("admin:%d:%s:user:%d", input.AssignedBy, input.OperationKey, input.UserID))
	payload, err := json.Marshal(struct {
		GroupID int64
		Days    int
		Notes   string
	}{input.GroupID, normalizeAssignValidityDays(input.ValidityDays), input.Notes})
	if err != nil {
		return nil, err
	}
	copy.AdminAssignmentFingerprint = HashIdempotencyKey(string(payload))
	sub, err := s.CreatePurchasedSubscription(ctx, &copy)
	if err != nil {
		return nil, err
	}
	if err := s.invalidateSubscriptionCaches(input.UserID, input.GroupID); err != nil {
		return nil, err
	}
	return sub, nil
}
