package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequest"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequestorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	InvoiceAwaitingPayment = "awaiting_payment"
	InvoicePending         = "pending"
	InvoiceIssued          = "issued"
	InvoiceCancelled       = "cancelled"
)

type InvoiceSelection struct {
	Selection string  `json:"selection"`
	OrderIDs  []int64 `json:"order_ids"`
}
type InvoiceOrderLine struct {
	ID        int64   `json:"id"`
	OrderNo   string  `json:"order_no"`
	OrderType string  `json:"order_type"`
	Name      string  `json:"name"`
	Amount    float64 `json:"amount"`
}
type InvoiceQuote struct {
	Orders      []InvoiceOrderLine `json:"orders"`
	Currency    string             `json:"currency"`
	BaseAmount  float64            `json:"base_amount"`
	ServiceFee  float64            `json:"service_fee"`
	TotalAmount float64            `json:"total_amount"`
	NetAmount   float64            `json:"net_amount"`
	TaxAmount   float64            `json:"tax_amount"`
	ItemName    string             `json:"item_name"`
	TaxRate     float64            `json:"tax_rate"`
	Tier        InvoiceFeeTier     `json:"tier"`
	Fingerprint string             `json:"fingerprint"`
}
type CreateInvoiceRequest struct {
	OrderIDs         []int64 `json:"order_ids"`
	TaxID            string  `json:"tax_id"`
	Title            string  `json:"title"`
	Email            string  `json:"email"`
	Remarks          string  `json:"remarks"`
	QuoteFingerprint string  `json:"quote_fingerprint"`
}
type InvoiceResult struct {
	ID          int64                `json:"id"`
	UserID      int64                `json:"user_id"`
	Status      string               `json:"status"`
	TaxID       string               `json:"tax_id"`
	Title       string               `json:"title"`
	Email       string               `json:"email"`
	Remarks     string               `json:"remarks"`
	Quote       InvoiceQuote         `json:"quote"`
	CreatedAt   time.Time            `json:"created_at"`
	ExpiresAt   time.Time            `json:"expires_at"`
	SubmittedAt *time.Time           `json:"submitted_at"`
	IssuedAt    *time.Time           `json:"issued_at"`
	IssuedBy    *int64               `json:"issued_by"`
	Payment     *CreateOrderResponse `json:"payment,omitempty"`
}
type OrderInvoiceSummary struct {
	ID          int64   `json:"id"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
}

func normalizeInvoiceIDs(ids []int64) ([]int64, error) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	if len(ids) == 0 || ids[0] <= 0 || len(slices.Compact(slices.Clone(ids))) != len(ids) {
		return nil, infraerrors.BadRequest("INVOICE_SELECTION_INVALID", "select complete orders without duplicates")
	}
	return ids, nil
}

func (s *PaymentService) QuoteInvoice(ctx context.Context, uid int64, selection InvoiceSelection) (*InvoiceQuote, error) {
	cfg, err := s.configService.GetInvoiceConfig(ctx)
	if err != nil {
		return nil, err
	}
	return quoteInvoice(ctx, s.entClient, uid, selection, *cfg, false)
}

func quoteInvoice(ctx context.Context, client *dbent.Client, uid int64, selection InvoiceSelection, cfg InvoiceConfig, lock bool) (*InvoiceQuote, error) {
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("INVOICE_DISABLED", "invoice applications are disabled")
	}
	q := client.PaymentOrder.Query().Where(paymentorder.UserIDEQ(uid), paymentorder.StatusEQ(OrderStatusCompleted), paymentorder.RefundAmountEQ(0), paymentorder.PaidAtNotNil(), paymentorder.OrderTypeIn(payment.OrderTypeBalance, payment.OrderTypeSubscription))
	q.Where(func(sel *sql.Selector) {
		occupied := sql.Select(invoicerequestorder.FieldOrderID).From(sql.Table(invoicerequestorder.Table)).Where(sql.IsNull(invoicerequestorder.FieldReleasedAt))
		sel.Where(sql.NotIn(sel.C(paymentorder.FieldID), occupied))
	})
	var ids []int64
	if selection.Selection == "selected" {
		var err error
		ids, err = normalizeInvoiceIDs(selection.OrderIDs)
		if err != nil {
			return nil, err
		}
		q.Where(paymentorder.IDIn(ids...))
	} else if selection.Selection != "all" || len(selection.OrderIDs) > 0 {
		return nil, infraerrors.BadRequest("INVOICE_SELECTION_INVALID", "invalid selection mode")
	}
	q.Order(dbent.Asc(paymentorder.FieldID))
	if lock && paymentAuditDialect(client) == dialect.Postgres {
		q.ForUpdate()
	}
	orders, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	if selection.Selection == "selected" && len(orders) != len(ids) {
		return nil, infraerrors.Conflict("INVOICE_ORDER_UNAVAILABLE", "one or more orders are no longer eligible")
	}
	planIDs := []int64{}
	for _, o := range orders {
		if o.PlanID != nil {
			planIDs = append(planIDs, *o.PlanID)
		}
	}
	slices.Sort(planIDs)
	planIDs = slices.Compact(planIDs)
	plans, err := client.SubscriptionPlan.Query().Where(subscriptionplan.IDIn(planIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	names := map[int64]string{}
	for _, p := range plans {
		names[p.ID] = p.Name
	}
	lines := []InvoiceOrderLine{}
	base := decimal.Zero
	for _, o := range orders {
		if PaymentOrderCurrency(o) != "CNY" || !invoiceMoneyValid(o.PayAmount, false) {
			if selection.Selection == "selected" {
				return nil, infraerrors.BadRequest("INVOICE_CURRENCY_INVALID", "only positive whole CNY payments can be invoiced")
			}
			continue
		}
		name := ""
		if o.PlanID != nil {
			name = names[*o.PlanID]
		}
		lines = append(lines, InvoiceOrderLine{ID: o.ID, OrderNo: o.OutTradeNo, OrderType: o.OrderType, Name: name, Amount: o.PayAmount})
		base = base.Add(decimal.NewFromFloat(o.PayAmount))
	}
	if len(lines) == 0 {
		return nil, infraerrors.BadRequest("INVOICE_NO_ORDERS", "no eligible orders to invoice")
	}
	tier, fee, total, net, tax, err := invoiceAmounts(base, cfg)
	if err != nil {
		return nil, err
	}
	result := &InvoiceQuote{Orders: lines, Currency: "CNY", BaseAmount: base.InexactFloat64(), ServiceFee: fee.InexactFloat64(), TotalAmount: total.InexactFloat64(), NetAmount: net.InexactFloat64(), TaxAmount: tax.InexactFloat64(), ItemName: cfg.ItemName, TaxRate: cfg.TaxRate, Tier: tier}
	result.Fingerprint = invoiceHash(struct {
		Config InvoiceConfig
		Quote  *InvoiceQuote
	}{cfg, result})
	return result, nil
}

func normalizeInvoiceRequest(req *CreateInvoiceRequest) error {
	var err error
	req.OrderIDs, err = normalizeInvoiceIDs(req.OrderIDs)
	if err != nil {
		return err
	}
	req.TaxID = strings.TrimSpace(req.TaxID)
	req.Title = strings.TrimSpace(req.Title)
	req.Email = strings.TrimSpace(req.Email)
	req.Remarks = strings.TrimSpace(req.Remarks)
	address, err := mail.ParseAddress(req.Email)
	if req.TaxID == "" || req.Title == "" || utf8.RuneCountInString(req.TaxID) > 64 || utf8.RuneCountInString(req.Title) > 200 || len(req.Email) > 254 || utf8.RuneCountInString(req.Remarks) > 1000 || err != nil || address.Address != req.Email || len(req.QuoteFingerprint) != 64 {
		return infraerrors.BadRequest("INVOICE_INFO_INVALID", "tax ID, title and valid email are required; remarks are optional")
	}
	return nil
}

func (s *PaymentService) CreateInvoice(ctx context.Context, uid int64, key string, req CreateInvoiceRequest) (*InvoiceResult, error) {
	if err := normalizeInvoiceRequest(&req); err != nil {
		return nil, err
	}
	key, err := NormalizeIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	operation := invoiceHash(fmt.Sprintf("invoice:%d:%s", uid, key))
	fingerprint := invoiceHash(req)
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	uq := tx.User.Query().Where(user.IDEQ(uid))
	if _, err = uq.Only(ctx); err != nil {
		return nil, err
	}
	existing, err := tx.InvoiceRequest.Query().Where(invoicerequest.OperationKeyEQ(operation)).Only(ctx)
	if err == nil {
		if existing.Fingerprint != fingerprint {
			return nil, ErrIdempotencyKeyConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.GetInvoice(ctx, uid, existing.ID)
	}
	if !dbent.IsNotFound(err) {
		return nil, err
	}
	sq := tx.Setting.Query().Where(setting.KeyEQ(SettingPaymentInvoiceConfig))
	if paymentAuditDialect(tx.Client()) == dialect.Postgres {
		// ponytail: serialize invoice creation on this configuration row; use versioned snapshots if volume warrants it.
		// Do not lock the user before source orders: existing refund transactions may lock them in the opposite order.
		sq.ForUpdate()
	}
	saved, err := sq.Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.Forbidden("INVOICE_DISABLED", "invoice applications are disabled")
	}
	if err != nil {
		return nil, err
	}
	// A concurrent identical request may have committed while the configuration lock was acquired.
	existing, err = tx.InvoiceRequest.Query().Where(invoicerequest.OperationKeyEQ(operation)).Only(ctx)
	if err == nil {
		if existing.Fingerprint != fingerprint {
			return nil, ErrIdempotencyKeyConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.GetInvoice(ctx, uid, existing.ID)
	}
	if !dbent.IsNotFound(err) {
		return nil, err
	}
	var cfg InvoiceConfig
	if err := json.Unmarshal([]byte(saved.Value), &cfg); err != nil {
		return nil, err
	}
	quote, err := quoteInvoice(ctx, tx.Client(), uid, InvoiceSelection{Selection: "selected", OrderIDs: req.OrderIDs}, cfg, true)
	if err != nil {
		return nil, err
	}
	if quote.Fingerprint != req.QuoteFingerprint {
		return nil, infraerrors.Conflict("INVOICE_QUOTE_CHANGED", "invoice quote changed; review the preview again")
	}
	paymentConfig, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !paymentConfig.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}
	minutes := paymentConfig.OrderTimeoutMin
	if minutes <= 0 {
		minutes = defaultOrderTimeoutMin
	}
	created, err := tx.InvoiceRequest.Create().SetUserID(uid).SetTaxID(req.TaxID).SetTitle(req.Title).SetEmail(req.Email).SetRemarks(req.Remarks).
		SetBaseAmount(quote.BaseAmount).SetServiceFee(quote.ServiceFee).SetTotalAmount(quote.TotalAmount).SetNetAmount(quote.NetAmount).SetTaxAmount(quote.TaxAmount).
		SetItemName(quote.ItemName).SetTaxRate(quote.TaxRate).SetFeeType(quote.Tier.Type).SetFeeValue(quote.Tier.Value).SetNillableFeeUpperAmount(quote.Tier.UpperAmount).
		SetOperationKey(operation).SetFingerprint(fingerprint).SetExpiresAt(time.Now().Add(time.Duration(minutes) * time.Minute)).Save(ctx)
	if err != nil {
		return nil, err
	}
	for _, line := range quote.Orders {
		_, err = tx.InvoiceRequestOrder.Create().SetInvoiceRequestID(created.ID).SetOrderID(line.ID).SetOrderNo(line.OrderNo).SetOrderType(line.OrderType).SetName(line.Name).SetAmount(line.Amount).Save(ctx)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetInvoice(ctx, uid, created.ID)
}

func invoiceResult(row *dbent.InvoiceRequest, lines []*dbent.InvoiceRequestOrder) InvoiceResult {
	quote := InvoiceQuote{Currency: row.Currency, BaseAmount: row.BaseAmount, ServiceFee: row.ServiceFee, TotalAmount: row.TotalAmount, NetAmount: row.NetAmount, TaxAmount: row.TaxAmount, ItemName: row.ItemName, TaxRate: row.TaxRate, Tier: InvoiceFeeTier{Type: row.FeeType, Value: row.FeeValue, UpperAmount: row.FeeUpperAmount}, Orders: []InvoiceOrderLine{}}
	for _, line := range lines {
		quote.Orders = append(quote.Orders, InvoiceOrderLine{ID: line.OrderID, OrderNo: line.OrderNo, OrderType: line.OrderType, Name: line.Name, Amount: line.Amount})
	}
	return InvoiceResult{ID: row.ID, UserID: row.UserID, Status: row.Status, TaxID: row.TaxID, Title: row.Title, Email: row.Email, Remarks: row.Remarks, Quote: quote, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, SubmittedAt: row.SubmittedAt, IssuedAt: row.IssuedAt, IssuedBy: row.IssuedBy}
}

func (s *PaymentService) GetInvoice(ctx context.Context, uid, id int64) (*InvoiceResult, error) {
	q := s.entClient.InvoiceRequest.Query().Where(invoicerequest.IDEQ(id))
	if uid > 0 {
		q.Where(invoicerequest.UserIDEQ(uid))
	}
	row, err := q.Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
	}
	if err != nil {
		return nil, err
	}
	lines, err := s.entClient.InvoiceRequestOrder.Query().Where(invoicerequestorder.InvoiceRequestIDEQ(id)).Order(dbent.Asc(invoicerequestorder.FieldOrderID)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := invoiceResult(row, lines)
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(id)).Only(ctx)
	if err == nil {
		result.Payment = invoicePaymentResponse(order)
	} else if !dbent.IsNotFound(err) {
		return nil, err
	}
	return &result, nil
}

func invoicePaymentResponse(o *dbent.PaymentOrder) *CreateOrderResponse {
	result := &CreateOrderResponse{InvoiceRequestID: o.InvoiceRequestID, OrderID: o.ID, Amount: o.Amount, PayAmount: o.PayAmount, Status: o.Status, PaymentType: o.PaymentType, OutTradeNo: o.OutTradeNo, Currency: PaymentOrderCurrency(o), ExpiresAt: o.ExpiresAt}
	if o.PayURL != nil {
		result.PayURL = *o.PayURL
	}
	if o.QrCode != nil {
		result.QRCode = *o.QrCode
	}
	return result
}

func (s *PaymentService) OrderInvoiceSummaries(ctx context.Context, orders []*dbent.PaymentOrder) (map[int64]OrderInvoiceSummary, error) {
	result := map[int64]OrderInvoiceSummary{}
	if len(orders) == 0 {
		return result, nil
	}
	ids, requests := []int64{}, []int64{}
	for _, o := range orders {
		ids = append(ids, o.ID)
		if o.InvoiceRequestID != nil {
			requests = append(requests, *o.InvoiceRequestID)
		}
	}
	links, err := s.entClient.InvoiceRequestOrder.Query().Where(invoicerequestorder.OrderIDIn(ids...), invoicerequestorder.ReleasedAtIsNil()).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		requests = append(requests, link.InvoiceRequestID)
	}
	if len(requests) == 0 {
		return result, nil
	}
	rows, err := s.entClient.InvoiceRequest.Query().Where(invoicerequest.IDIn(requests...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int64]OrderInvoiceSummary{}
	for _, r := range rows {
		byID[r.ID] = OrderInvoiceSummary{ID: r.ID, Status: r.Status, TotalAmount: r.TotalAmount}
	}
	for _, link := range links {
		result[link.OrderID] = byID[link.InvoiceRequestID]
	}
	for _, o := range orders {
		if o.InvoiceRequestID != nil {
			result[o.ID] = byID[*o.InvoiceRequestID]
		}
	}
	return result, nil
}
