package service

import (
	"context"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeySubscriptionFreezeEnabled = "subscription_freeze_enabled"

var (
	ErrSubscriptionFrozen         = infraerrors.Forbidden("SUBSCRIPTION_FROZEN", "subscription is frozen")
	ErrSubscriptionFreezeDisabled = infraerrors.Forbidden("SUBSCRIPTION_FREEZE_DISABLED", "subscription freezing is disabled")
)

func (s *SubscriptionService) SetUserSubscriptionFrozen(ctx context.Context, userID, id int64, freeze bool) (*UserSubscription, error) {
	if s.entClient == nil {
		return nil, ErrBillingServiceUnavailable
	}
	var ownerGroup int64
	err := s.withSubscriptionOwnerTx(ctx, userID, func(txCtx context.Context) error {
		sub, err := s.userSubRepo.GetByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if sub.UserID != userID || sub.DeletedAt != nil {
			return ErrSubscriptionNotFound
		}
		ownerGroup = sub.GroupID
		client := dbent.TxFromContext(txCtx).Client()
		now := time.Now()
		if s.now != nil {
			now = s.now()
		}
		now = now.UTC().Truncate(time.Microsecond)
		if freeze {
			if sub.FrozenAt != nil {
				return nil
			}
			policy, err := client.Setting.Query().Where(setting.KeyEQ(SettingKeySubscriptionFreezeEnabled)).Only(txCtx)
			if dbent.IsNotFound(err) {
				return ErrSubscriptionFreezeDisabled
			}
			if err != nil {
				return err
			}
			if policy.Value != "true" {
				return ErrSubscriptionFreezeDisabled
			}
			if sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(now) || sub.StartsAt.After(now) {
				return ErrSubscriptionInvalid
			}
			if err := s.checkAndResetWindowsAt(txCtx, sub, now); err != nil {
				return err
			}
			_, err = client.UserSubscription.UpdateOneID(id).SetFrozenAt(now).Save(txCtx)
			return err
		}
		// Thaw never checks the global switch and never changes admin status.
		if sub.FrozenAt == nil {
			return nil
		}
		delta := now.Sub(*sub.FrozenAt).Truncate(time.Microsecond)
		if delta < 0 {
			return infraerrors.Conflict("SUBSCRIPTION_CLOCK_CHANGED", "server time precedes the freeze time")
		}
		if sub.FrozenDurationUS > math.MaxInt64/int64(time.Microsecond)-delta.Microseconds() || sub.ExpiresAt.Add(delta).After(MaxExpiresAt) {
			return infraerrors.BadRequest("SUBSCRIPTION_TIME_LIMIT", "subscription time exceeds the supported range")
		}
		update := client.UserSubscription.UpdateOneID(id).ClearFrozenAt().SetFrozenDurationUs(sub.FrozenDurationUS + delta.Microseconds()).SetExpiresAt(sub.ExpiresAt.Add(delta))
		if sub.DailyWindowStart != nil {
			update.SetDailyWindowStart(sub.DailyWindowStart.Add(delta))
		}
		if sub.WeeklyWindowStart != nil {
			update.SetWeeklyWindowStart(sub.WeeklyWindowStart.Add(delta))
		}
		if sub.MonthlyWindowStart != nil {
			update.SetMonthlyWindowStart(sub.MonthlyWindowStart.Add(delta))
		}
		_, err = update.Save(txCtx)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := s.invalidateSubscriptionCaches(userID, ownerGroup); err != nil {
		return nil, err
	}
	return s.userSubRepo.GetByID(ctx, id)
}
