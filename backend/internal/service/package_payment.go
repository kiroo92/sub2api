package service

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *PaymentService) SetPackageService(packages *PackageService) { s.packageSvc = packages }

// Package delivery is one product branch of the ordinary verified-payment
// fulfillment flow. Group settlement is owned by PackageService, not payment.
func (s *PaymentService) executePackageFulfillment(ctx context.Context, o *dbent.PaymentOrder) error {
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if s.packageSvc == nil {
		return infraerrors.ServiceUnavailable("PACKAGES_UNAVAILABLE", "package service unavailable")
	}
	if o.PaidAt == nil || psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "package order is not a verified payment")
	}
	lease, err := s.acquirePaymentFulfillmentLease(ctx, o)
	if err != nil || lease == nil {
		return err
	}
	if err = s.packageSvc.fulfill(ctx, o); err != nil {
		s.markFailed(ctx, o.ID, lease, err)
		return err
	}
	return s.markCompleted(ctx, o, lease, "PACKAGE_SUCCESS")
}
