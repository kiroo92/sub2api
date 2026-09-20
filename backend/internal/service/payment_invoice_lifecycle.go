package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequest"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequestorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type UnpaidInvoice struct {
	ID          int64     `json:"id"`
	TotalAmount float64   `json:"total_amount"`
	ServiceFee  float64   `json:"service_fee"`
	CreatedAt   time.Time `json:"created_at"`
}

// Listing must be read-only: a page visit is not consent to cancel or recreate a payment.
func (s *PaymentService) ListUnpaidInvoices(ctx context.Context, uid int64) ([]UnpaidInvoice, error) {
	rows, err := s.entClient.InvoiceRequest.Query().Where(invoicerequest.UserIDEQ(uid), invoicerequest.StatusEQ(InvoiceAwaitingPayment)).Order(dbent.Desc(invoicerequest.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]UnpaidInvoice, 0, len(rows))
	for _, row := range rows {
		result = append(result, UnpaidInvoice{ID: row.ID, TotalAmount: row.TotalAmount, ServiceFee: row.ServiceFee, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

func (s *PaymentService) CancelInvoice(ctx context.Context, uid, id int64) (*InvoiceResult, error) {
	row, err := s.entClient.InvoiceRequest.Query().Where(invoicerequest.IDEQ(id), invoicerequest.UserIDEQ(uid)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
	}
	if err != nil {
		return nil, err
	}
	if row.Status == InvoiceCancelled {
		return s.GetInvoice(ctx, uid, id)
	}
	if row.Status != InvoiceAwaitingPayment {
		return nil, infraerrors.Conflict("INVOICE_ALREADY_PAID", "invoice service fee is paid; cancellation is unavailable")
	}
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(id)).Only(ctx)
	var cancellationErr error
	if dbent.IsNotFound(err) {
		// releaseInvoice rechecks the payment binding under the same lock as creation.
		cancellationErr = s.releaseInvoice(ctx, id, 0)
	} else if err != nil {
		return nil, err
	} else {
		if order.PaidAt == nil && order.Status != OrderStatusCompleted {
			// Also permit an explicit cancellation of failed/expired attempts; the
			// existing provider check and subsequent reconciliation still require final closure.
			_, cancellationErr = s.cancelCore(ctx, order, OrderStatusCancelled, fmt.Sprintf("user:%d", uid), "user cancelled invoice application")
		}
		if cancellationErr == nil {
			cancellationErr = s.reconcileInvoice(ctx, row)
		}
	}
	current, err := s.GetInvoice(ctx, uid, id)
	if err != nil {
		return nil, err
	}
	switch current.Status {
	case InvoiceCancelled:
		return current, nil
	case InvoicePending, InvoiceIssued:
		return nil, infraerrors.Conflict("INVOICE_ALREADY_PAID", "invoice service fee is paid; cancellation is unavailable")
	default:
		return nil, infraerrors.Conflict("INVOICE_CANCEL_UNCONFIRMED", "payment closure has not been confirmed; please check again later").WithCause(cancellationErr)
	}
}

func lockInvoice(ctx context.Context, client *dbent.Client, id int64) (*dbent.InvoiceRequest, error) {
	q := client.InvoiceRequest.Query().Where(invoicerequest.IDEQ(id))
	if paymentAuditDialect(client) == dialect.Postgres {
		q.ForUpdate()
	}
	return q.Only(ctx)
}

func (s *PaymentService) prepareInvoicePayment(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error) {
	if req.InvoiceRequestID <= 0 || req.PlanID != 0 || req.CouponCode != "" {
		return nil, infraerrors.BadRequest("INVOICE_PAYMENT_INVALID", "invoice payment requires only an owned invoice application")
	}
	row, err := s.entClient.InvoiceRequest.Query().Where(invoicerequest.IDEQ(req.InvoiceRequestID), invoicerequest.UserIDEQ(req.UserID)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
	}
	if err != nil {
		return nil, err
	}
	if req.Amount != 0 && req.Amount != row.ServiceFee {
		return nil, infraerrors.BadRequest("INVOICE_AMOUNT_INVALID", "service fee must match the saved invoice")
	}
	if req.ExpectedPayAmount != nil && *req.ExpectedPayAmount != row.ServiceFee {
		return nil, infraerrors.Conflict("INVOICE_QUOTE_CHANGED", "review the saved invoice fee")
	}
	existing, err := s.entClient.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(row.ID)).Only(ctx)
	if err == nil {
		return invoicePaymentResponse(existing), nil
	}
	if !dbent.IsNotFound(err) {
		return nil, err
	}
	if row.Status != InvoiceAwaitingPayment || !row.ExpiresAt.After(time.Now()) {
		return nil, infraerrors.Conflict("INVOICE_NOT_PAYABLE", "invoice application is no longer payable")
	}
	req.Amount = row.ServiceFee
	req.invoice = row
	return nil, nil
}

func (s *PaymentService) lockInvoiceForPayment(ctx context.Context, tx *dbent.Tx, req CreateOrderRequest) error {
	row, err := lockInvoice(ctx, tx.Client(), req.InvoiceRequestID)
	if err != nil {
		return err
	}
	if row.UserID != req.UserID || row.Status != InvoiceAwaitingPayment || !row.ExpiresAt.After(time.Now()) || row.ServiceFee != req.Amount {
		return infraerrors.Conflict("INVOICE_NOT_PAYABLE", "invoice application is no longer payable")
	}
	exists, err := tx.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(row.ID)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return infraerrors.Conflict("INVOICE_PAYMENT_EXISTS", "invoice payment already exists")
	}
	return nil
}

// The only invoice fulfillment path: payment evidence and the invoice submission commit together.
func (s *PaymentService) completeInvoicePayment(ctx context.Context, oid int64, paid *float64, tradeNo string) error {
	initial, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return err
	}
	if initial.OrderType != payment.OrderTypeInvoiceFee || initial.InvoiceRequestID == nil {
		return infraerrors.BadRequest("INVOICE_PAYMENT_INVALID", "not an invoice fee payment")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	invoice, err := lockInvoice(ctx, tx.Client(), *initial.InvoiceRequestID)
	if err != nil {
		return err
	}
	q := tx.PaymentOrder.Query().Where(paymentorder.IDEQ(oid))
	if paymentAuditDialect(tx.Client()) == dialect.Postgres {
		q.ForUpdate()
	}
	order, err := q.Only(ctx)
	if err != nil {
		return err
	}
	if invoice.Status == InvoiceCancelled {
		_ = tx.Rollback()
		s.writeAuditLog(ctx, oid, "INVOICE_PAYMENT_RECONCILIATION_REQUIRED", "system", map[string]any{"invoiceID": invoice.ID})
		return infraerrors.Conflict("INVOICE_PAYMENT_RECONCILIATION_REQUIRED", "payment arrived after confirmed closure")
	}
	if order.UserID != invoice.UserID || order.InvoiceRequestID == nil || *order.InvoiceRequestID != invoice.ID || order.OrderType != payment.OrderTypeInvoiceFee || order.Amount != invoice.ServiceFee || order.PayAmount != invoice.ServiceFee || PaymentOrderCurrency(order) != "CNY" || psIsRefundStatus(order.Status) {
		return infraerrors.BadRequest("INVOICE_PAYMENT_INVALID", "invoice payment does not match its application")
	}
	if paid != nil && (!invoiceMoneyValid(*paid, false) || *paid != invoice.ServiceFee) {
		return infraerrors.BadRequest("INVOICE_AMOUNT_INVALID", "paid amount does not match invoice service fee")
	}
	if paid == nil && order.PaidAt == nil {
		return infraerrors.BadRequest("PAYMENT_NOT_CONFIRMED", "invoice fee payment has not been confirmed")
	}
	if order.Status == OrderStatusCompleted && (invoice.Status == InvoicePending || invoice.Status == InvoiceIssued) {
		return nil
	}
	now := time.Now()
	update := tx.PaymentOrder.UpdateOneID(oid).SetStatus(OrderStatusCompleted).SetCompletedAt(now).ClearFailedAt().ClearFailedReason()
	if order.PaidAt == nil {
		update.SetPaidAt(now).SetPaymentTradeNo(tradeNo)
	}
	if _, err := update.Save(ctx); err != nil {
		return err
	}
	if invoice.Status == InvoiceAwaitingPayment {
		if _, err := tx.InvoiceRequest.UpdateOneID(invoice.ID).SetStatus(InvoicePending).SetSubmittedAt(now).Save(ctx); err != nil {
			return err
		}
	}
	_, err = tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(oid, 10)).SetAction("INVOICE_SUBMITTED").SetOperator("system").SetDetail(fmt.Sprintf(`{"invoice_id":%d}`, invoice.ID)).Save(ctx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Caller must prove no provider call occurred, or that the bound provider order is closed.
func (s *PaymentService) releaseInvoice(ctx context.Context, id int64, paymentID int64) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	row, err := lockInvoice(ctx, tx.Client(), id)
	if err != nil {
		return err
	}
	if row.Status != InvoiceAwaitingPayment {
		return nil
	}
	q := tx.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(id))
	if paymentAuditDialect(tx.Client()) == dialect.Postgres {
		q.ForUpdate()
	}
	order, err := q.Only(ctx)
	if err == nil {
		if order.ID != paymentID || order.PaidAt != nil || order.Status == OrderStatusCompleted {
			return nil
		}
	} else if !dbent.IsNotFound(err) {
		return err
	} else if paymentID != 0 {
		return nil
	}
	if _, err := tx.InvoiceRequest.UpdateOneID(id).SetStatus(InvoiceCancelled).Save(ctx); err != nil {
		return err
	}
	if _, err := tx.InvoiceRequestOrder.Update().Where(invoicerequestorder.InvoiceRequestIDEQ(id), invoicerequestorder.ReleasedAtIsNil()).SetReleasedAt(time.Now()).Save(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PaymentService) reconcileInvoice(ctx context.Context, row *dbent.InvoiceRequest) error {
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(row.ID)).Only(ctx)
	if dbent.IsNotFound(err) {
		if !row.ExpiresAt.After(time.Now()) {
			return s.releaseInvoice(ctx, row.ID, 0)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if order.PaidAt != nil {
		return s.completeInvoicePayment(ctx, order.ID, nil, "")
	}
	provider, err := s.getOrderProvider(ctx, order)
	if err != nil {
		return err
	}
	ref := paymentOrderQueryReference(order, provider)
	if ref == "" {
		return nil
	}
	result, err := provider.QueryOrder(ctx, ref)
	if err != nil || result == nil {
		return err
	}
	if result.Status == payment.ProviderStatusPaid {
		return s.HandlePaymentNotification(ctx, &payment.PaymentNotification{OrderID: order.OutTradeNo, TradeNo: result.TradeNo, Amount: result.Amount, Status: payment.NotificationStatusSuccess, Metadata: result.Metadata}, provider.ProviderKey())
	}
	if result.Closed {
		return s.releaseInvoice(ctx, row.ID, order.ID)
	}
	if order.Status != OrderStatusPending || !order.ExpiresAt.After(time.Now()) {
		// Do not attempt closure until a create response was persisted: a timed-out create may still run.
		if order.PaymentTradeNo != "" || order.PayURL != nil || order.QrCode != nil {
			if cp, ok := provider.(payment.CancelableProvider); ok {
				if err := cp.CancelPayment(ctx, ref); err != nil {
					return err
				}
				closed, err := provider.QueryOrder(ctx, ref)
				if err != nil {
					return err
				}
				if closed != nil && closed.Status == payment.ProviderStatusPaid {
					return s.HandlePaymentNotification(ctx, &payment.PaymentNotification{OrderID: order.OutTradeNo, TradeNo: closed.TradeNo, Amount: closed.Amount, Status: payment.NotificationStatusSuccess, Metadata: closed.Metadata}, provider.ProviderKey())
				}
				if closed != nil && closed.Closed {
					return s.releaseInvoice(ctx, row.ID, order.ID)
				}
			}
		}
	}
	return nil
}

func (s *PaymentService) ReconcileInvoicePayments(ctx context.Context) error {
	q := s.entClient.InvoiceRequest.Query().Where(invoicerequest.StatusEQ(InvoiceAwaitingPayment))
	rows, err := q.Clone().Where(invoicerequest.IDGT(s.invoiceReconcileCursor.Load())).Order(dbent.Asc(invoicerequest.FieldID)).Limit(50).All(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		s.invoiceReconcileCursor.Store(0)
		return nil
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := s.reconcileInvoice(attempt, row)
		cancel()
		s.invoiceReconcileCursor.Store(row.ID)
		if err != nil {
			slog.Warn("invoice payment reconciliation", "invoiceID", row.ID, "error", err)
		}
	}
	return nil
}
