package service

import (
	"context"
	"log/slog"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/subscriptiondiscountcode"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func lockPaymentDiscount(ctx context.Context, client *dbent.Client, id int64) error {
	q := client.SubscriptionDiscountCode.Query().Where(subscriptiondiscountcode.IDEQ(id))
	if paymentAuditDialect(client) == dialect.Postgres {
		q.ForUpdate()
	}
	_, err := q.Only(ctx)
	return err
}

// Caller must prove either no provider call occurred or the provider closed the order.
func (s *PaymentService) releasePaymentDiscount(ctx context.Context, order *dbent.PaymentOrder) error {
	if order.DiscountCodeID == nil {
		return nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockPaymentDiscount(ctx, tx.Client(), *order.DiscountCodeID); err != nil {
		return err
	}
	_, err = tx.PaymentOrder.Update().Where(paymentorder.IDEQ(order.ID), paymentorder.PaidAtIsNil(),
		paymentorder.DiscountStateIn(discountCreating, discountReserved),
		paymentorder.StatusIn(OrderStatusPending, OrderStatusCancelled, OrderStatusExpired, OrderStatusFailed)).
		SetDiscountState(discountReleased).Save(ctx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PaymentService) reconcileDiscountOrder(ctx context.Context, order *dbent.PaymentOrder) error {
	if order.DiscountCodeID == nil || order.DiscountState != discountCreating && order.DiscountState != discountReserved {
		return nil
	}
	prov, err := s.getOrderProvider(ctx, order)
	if err != nil {
		return err
	}
	ref := paymentOrderQueryReference(order, prov)
	if ref == "" {
		return nil
	}
	result, err := prov.QueryOrder(ctx, ref)
	if err != nil || result == nil {
		return err
	}
	if result.Status == payment.ProviderStatusPaid {
		return s.HandlePaymentNotification(ctx, &payment.PaymentNotification{OrderID: order.OutTradeNo, TradeNo: result.TradeNo, Amount: result.Amount, Status: payment.NotificationStatusSuccess, Metadata: result.Metadata}, prov.ProviderKey())
	}
	if result.Closed {
		return s.releasePaymentDiscount(ctx, order)
	}
	// Only close an existing upstream order. A not-found response is not final,
	// especially while a timed-out CreatePayment may still be completing.
	if cp, ok := prov.(payment.CancelableProvider); ok && order.DiscountState == discountReserved {
		if err := cp.CancelPayment(ctx, ref); err != nil {
			return err
		}
		result, err = prov.QueryOrder(ctx, ref)
		if err != nil {
			return err
		}
		if result != nil && result.Closed {
			return s.releasePaymentDiscount(ctx, order)
		}
	}
	return nil
}

// Reuse payment reconciliation and page fairly past orders whose providers cannot close.
func (s *PaymentService) ReconcileDiscountOrders(ctx context.Context) error {
	q := s.entClient.PaymentOrder.Query().Where(paymentorder.DiscountStateIn(discountCreating, discountReserved),
		paymentorder.PaidAtIsNil(), paymentorder.StatusIn(OrderStatusCancelled, OrderStatusExpired, OrderStatusFailed))
	orders, err := q.Clone().Where(paymentorder.IDGT(s.discountReconcileCursor.Load())).Order(dbent.Asc(paymentorder.FieldID)).Limit(50).All(ctx)
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		s.discountReconcileCursor.Store(0)
		orders, err = q.Order(dbent.Asc(paymentorder.FieldID)).Limit(50).All(ctx)
		if err != nil {
			return err
		}
	}
	for _, order := range orders {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := s.reconcileDiscountOrder(attempt, order)
		cancel()
		s.discountReconcileCursor.Store(order.ID)
		if err != nil {
			slog.Warn("discount payment reconciliation", "orderID", order.ID, "error", err)
		}
	}
	return nil
}
