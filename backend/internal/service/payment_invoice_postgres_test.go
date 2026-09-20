//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequestorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestInvoicePostgres(t *testing.T) {
	dsn := os.Getenv("INVOICE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set INVOICE_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("invoice_test_%d", time.Now().UnixNano())
	_, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, e := base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, e)
		require.NoError(t, base.Close())
	})
	target, err := url.Parse(dsn)
	require.NoError(t, err)
	query := target.Query()
	query.Set("search_path", schema)
	target.RawQuery = query.Encode()
	db, err := sql.Open("postgres", target.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.Schema.Create(ctx))
	_, err = db.ExecContext(ctx, `ALTER TABLE payment_orders DROP COLUMN invoice_request_id; DROP TABLE invoice_request_orders; DROP TABLE invoice_requests`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("245_order_invoicing.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}

	owner, err := client.User.Create().SetEmail("invoice-owner@example.com").SetPasswordHash("test").SetUsername("invoice").SetBalance(20).Save(ctx)
	require.NoError(t, err)
	other, err := client.User.Create().SetEmail("invoice-other@example.com").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	cfg := InvoiceConfig{Enabled: true, ItemName: "技术服务费", TaxRate: 3, Tiers: []InvoiceFeeTier{{nil, "fixed", 38}}}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	_, err = client.Setting.Create().SetKey(SettingPaymentInvoiceConfig).SetValue(string(raw)).Save(ctx)
	require.NoError(t, err)
	settings := &paymentConfigSettingRepoStub{values: map[string]string{SettingPaymentInvoiceConfig: string(raw), SettingPaymentEnabled: "true"}}
	service := &PaymentService{entClient: client, configService: NewPaymentConfigService(client, settings, nil)}
	user := &User{ID: owner.ID, Email: owner.Email, Username: owner.Username}
	sequence := 0
	source := func(uid int64, amount float64) *dbent.PaymentOrder {
		sequence++
		row, e := client.PaymentOrder.Create().SetUserID(uid).SetUserEmail("invoice@example.com").SetUserName("invoice").SetAmount(999).SetPayAmount(amount).SetRechargeCode("source").SetOutTradeNo(fmt.Sprintf("source_%d", sequence)).SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("paid").SetOrderType(payment.OrderTypeBalance).SetStatus(OrderStatusCompleted).SetExpiresAt(time.Now().Add(time.Hour)).SetPaidAt(time.Now()).SetClientIP("127.0.0.1").SetSrcHost("test").Save(ctx)
		require.NoError(t, e)
		return row
	}
	for i := 0; i < 30; i++ {
		source(owner.ID, 10)
	}
	source(owner.ID, 11.5)
	source(other.ID, 888)
	unpaid := source(owner.ID, 100)
	_, err = client.PaymentOrder.UpdateOneID(unpaid.ID).SetStatus(OrderStatusPending).ClearPaidAt().Save(ctx)
	require.NoError(t, err)
	foreignCurrency := source(owner.ID, 50)
	_, err = client.PaymentOrder.UpdateOneID(foreignCurrency.ID).SetProviderSnapshot(map[string]any{"currency": "USD"}).Save(ctx)
	require.NoError(t, err)
	quote, err := service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "all"})
	require.NoError(t, err)
	require.Len(t, quote.Orders, 31, "all eligible orders, not one UI page")
	require.Equal(t, 311.5, quote.BaseAmount)
	require.Equal(t, 349.5, quote.TotalAmount)
	ids := []int64{}
	for _, line := range quote.Orders {
		ids = append(ids, line.ID)
	}
	req := CreateInvoiceRequest{OrderIDs: ids, TaxID: "001234567890", Title: "Test buyer", Email: "billing@example.com", QuoteFingerprint: quote.Fingerprint}
	_, err = db.ExecContext(ctx, `UPDATE settings SET value = $1 WHERE key = $2`, strings.Replace(string(raw), "技术服务费", "不同项目", 1), SettingPaymentInvoiceConfig)
	require.NoError(t, err)
	_, err = service.CreateInvoice(ctx, owner.ID, "stale-quote", req)
	require.Error(t, err, "a changed invoice item requires a new preview")
	_, err = db.ExecContext(ctx, `UPDATE settings SET value = $1 WHERE key = $2`, string(raw), SettingPaymentInvoiceConfig)
	require.NoError(t, err)
	_, err = service.QuoteInvoice(ctx, other.ID, InvoiceSelection{Selection: "selected", OrderIDs: ids})
	require.Error(t, err)

	type attempt struct {
		result *InvoiceResult
		err    error
		key    string
	}
	results := make(chan attempt, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			key := fmt.Sprintf("invoice-create-%d", i)
			result, e := service.CreateInvoice(ctx, owner.ID, key, req)
			results <- attempt{result, e, key}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	var created *InvoiceResult
	var operation string
	success := 0
	for attempt := range results {
		if attempt.err == nil {
			success++
			created = attempt.result
			operation = attempt.key
		}
	}
	require.Equal(t, 1, success)
	require.NotNil(t, created)
	replay, err := service.CreateInvoice(ctx, owner.ID, operation, req)
	require.NoError(t, err)
	require.Equal(t, created.ID, replay.ID)
	changed := req
	changed.Title = "Different buyer"
	_, err = service.CreateInvoice(ctx, owner.ID, operation, changed)
	require.ErrorIs(t, err, ErrIdempotencyKeyConflict)
	_, err = service.GetInvoice(ctx, other.ID, created.ID)
	require.Error(t, err)
	_, err = service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "selected", OrderIDs: ids})
	require.Error(t, err)
	listed, count, err := service.ListInvoices(ctx, InvoiceListParams{})
	require.NoError(t, err)
	require.Zero(t, count)
	require.Empty(t, listed)
	require.Error(t, service.MarkInvoicesIssued(ctx, owner.ID, []int64{created.ID}))

	payReq := CreateOrderRequest{UserID: owner.ID, OrderType: payment.OrderTypeInvoiceFee, InvoiceRequestID: created.ID, PaymentType: payment.TypeAlipay}
	existing, err := service.prepareInvoicePayment(ctx, &payReq)
	require.NoError(t, err)
	require.Nil(t, existing)
	require.Equal(t, 38.0, payReq.Amount)
	paymentResults := make(chan *dbent.PaymentOrder, 2)
	start = make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			row, e := service.createOrderInTx(ctx, payReq, user, nil, &PaymentConfig{MaxPendingOrders: 100}, 38, 38, 0, 38, nil)
			if e == nil {
				paymentResults <- row
			}
		}()
	}
	close(start)
	wg.Wait()
	close(paymentResults)
	var fee *dbent.PaymentOrder
	success = 0
	for row := range paymentResults {
		fee = row
		success++
	}
	require.Equal(t, 1, success)
	require.Error(t, service.RetryFulfillment(ctx, fee.ID))
	require.Error(t, service.ExecuteBalanceFulfillment(ctx, fee.ID))
	require.Error(t, service.ExecuteSubscriptionFulfillment(ctx, fee.ID))
	_, err = client.PaymentOrder.UpdateOneID(fee.ID).SetStatus(OrderStatusFailed).Save(ctx)
	require.NoError(t, err)
	notify := &payment.PaymentNotification{OrderID: fee.OutTradeNo, TradeNo: "invoice-fee-paid", Amount: 38, Status: payment.NotificationStatusSuccess}
	notifications := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			notifications <- service.HandlePaymentNotification(ctx, notify, payment.TypeAlipay)
		}()
	}
	wg.Wait()
	close(notifications)
	for e := range notifications {
		require.NoError(t, e)
	}
	require.NoError(t, service.HandlePaymentNotification(ctx, notify, payment.TypeAlipay))
	after, err := client.User.Get(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, 20.0, after.Balance)
	n, err := client.UserSubscription.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = client.RedeemCode.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = client.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ("INVOICE_SUBMITTED")).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	_, _, err = service.PrepareRefund(ctx, fee.ID, 38, "not supported", true, true)
	require.Error(t, err)
	require.NoError(t, service.MarkInvoicesIssued(ctx, owner.ID, []int64{created.ID}))
	require.NoError(t, service.MarkInvoicesIssued(ctx, owner.ID, []int64{created.ID}))
	record, err := service.GetInvoice(ctx, owner.ID, created.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceIssued, record.Status)
	require.NotNil(t, record.IssuedAt)
	require.NoError(t, service.releaseInvoice(ctx, created.ID, fee.ID))
	n, err = client.InvoiceRequestOrder.Query().Where(invoicerequestorder.InvoiceRequestIDEQ(created.ID), invoicerequestorder.ReleasedAtNotNil()).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	summaries, err := service.OrderInvoiceSummaries(ctx, []*dbent.PaymentOrder{fee, source(other.ID, 1)})
	require.NoError(t, err)
	require.Equal(t, InvoiceIssued, summaries[fee.ID].Status)
	listed, count, err = service.ListInvoices(ctx, InvoiceListParams{Search: "001234", Status: InvoiceIssued})
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, 349.5, listed[0].Quote.TotalAmount)

	// Cancellation without final provider closure must retain the source order.
	extra := source(owner.ID, 25)
	q2, err := service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "selected", OrderIDs: []int64{extra.ID}})
	require.NoError(t, err)
	r2 := req
	r2.OrderIDs = []int64{extra.ID}
	r2.QuoteFingerprint = q2.Fingerprint
	application, err := service.CreateInvoice(ctx, owner.ID, "cancel-invoice", r2)
	require.NoError(t, err)
	nextReq := CreateOrderRequest{UserID: owner.ID, OrderType: payment.OrderTypeInvoiceFee, InvoiceRequestID: application.ID, PaymentType: payment.TypeAlipay}
	_, err = service.prepareInvoicePayment(ctx, &nextReq)
	require.NoError(t, err)
	pending, err := service.createOrderInTx(ctx, nextReq, user, nil, &PaymentConfig{MaxPendingOrders: 100}, 38, 38, 0, 38, nil)
	require.NoError(t, err)
	pending, err = client.PaymentOrder.UpdateOneID(pending.ID).SetStatus(OrderStatusCancelled).Save(ctx)
	require.NoError(t, err)
	provider := &paymentOrderLifecycleQueryProvider{resp: &payment.QueryOrderResponse{Status: payment.ProviderStatusPending, TradeNo: pending.OutTradeNo}}
	service.registry = payment.NewRegistry()
	service.registry.Register(provider)
	service.providersLoaded = true
	_, err = client.PaymentOrder.UpdateOneID(pending.ID).SetPaymentType(provider.ProviderKey()).Save(ctx)
	require.NoError(t, err)
	saved, err := client.InvoiceRequest.Get(ctx, application.ID)
	require.NoError(t, err)
	require.NoError(t, service.reconcileInvoice(ctx, saved))
	_, err = service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "selected", OrderIDs: []int64{extra.ID}})
	require.Error(t, err)
	provider.resp.Closed = true
	require.NoError(t, service.reconcileInvoice(ctx, saved))
	require.NoError(t, service.reconcileInvoice(ctx, saved))
	_, err = service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "selected", OrderIDs: []int64{extra.ID}})
	require.NoError(t, err)
	require.Error(t, service.toPaid(ctx, pending, "late", 38, payment.TypeAlipay))
	current, err := client.PaymentOrder.Get(ctx, pending.ID)
	require.NoError(t, err)
	require.Nil(t, current.PaidAt)
	// A closure release racing a confirmed payment cannot both free and invoice the same source.
	q3, err := service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "selected", OrderIDs: []int64{extra.ID}})
	require.NoError(t, err)
	r2.QuoteFingerprint = q3.Fingerprint
	raceApplication, err := service.CreateInvoice(ctx, owner.ID, "race-invoice", r2)
	require.NoError(t, err)
	raceReq := CreateOrderRequest{UserID: owner.ID, OrderType: payment.OrderTypeInvoiceFee, InvoiceRequestID: raceApplication.ID, PaymentType: payment.TypeAlipay}
	_, err = service.prepareInvoicePayment(ctx, &raceReq)
	require.NoError(t, err)
	racePayment, err := service.createOrderInTx(ctx, raceReq, user, nil, &PaymentConfig{MaxPendingOrders: 100}, 38, 38, 0, 38, nil)
	require.NoError(t, err)
	start = make(chan struct{})
	outcomes := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		outcomes <- service.releaseInvoice(ctx, raceApplication.ID, racePayment.ID)
	}()
	go func() {
		defer wg.Done()
		<-start
		outcomes <- service.toPaid(ctx, racePayment, "racing-paid", 38, payment.TypeAlipay)
	}()
	close(start)
	wg.Wait()
	close(outcomes)
	for range outcomes {
	}
	raceRecord, err := service.GetInvoice(ctx, owner.ID, raceApplication.ID)
	require.NoError(t, err)
	racePayment, err = client.PaymentOrder.Get(ctx, racePayment.ID)
	require.NoError(t, err)
	n, err = client.InvoiceRequestOrder.Query().Where(invoicerequestorder.InvoiceRequestIDEQ(raceApplication.ID), invoicerequestorder.ReleasedAtNotNil()).Count(ctx)
	require.NoError(t, err)
	if raceRecord.Status == InvoiceCancelled {
		require.Nil(t, racePayment.PaidAt)
		require.Equal(t, 1, n)
	} else {
		require.Equal(t, InvoicePending, raceRecord.Status)
		require.NotNil(t, racePayment.PaidAt)
		require.Zero(t, n)
	}
	// Opening the page only lists owned unpaid applications; cancellation is explicit.
	newDraft := func(key string) *InvoiceResult {
		origin := source(owner.ID, 12)
		preview, e := service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "selected", OrderIDs: []int64{origin.ID}})
		require.NoError(t, e)
		input := req
		input.OrderIDs = []int64{origin.ID}
		input.QuoteFingerprint = preview.Fingerprint
		application, e := service.CreateInvoice(ctx, owner.ID, key, input)
		require.NoError(t, e)
		return application
	}
	newFee := func(application *InvoiceResult) *dbent.PaymentOrder {
		input := CreateOrderRequest{UserID: owner.ID, OrderType: payment.OrderTypeInvoiceFee, InvoiceRequestID: application.ID, PaymentType: payment.TypeAlipay}
		_, e := service.prepareInvoicePayment(ctx, &input)
		require.NoError(t, e)
		row, e := service.createOrderInTx(ctx, input, user, nil, &PaymentConfig{MaxPendingOrders: 100}, 38, 38, 0, 38, nil)
		require.NoError(t, e)
		return row
	}
	draft := newDraft("manual-no-payment")
	queryCount, cancelCount := provider.queryCalls, provider.cancelCalls
	unpaidApplications, err := service.ListUnpaidInvoices(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, unpaidApplications, 1)
	require.Equal(t, draft.ID, unpaidApplications[0].ID)
	otherApplications, err := service.ListUnpaidInvoices(ctx, other.ID)
	require.NoError(t, err)
	require.Empty(t, otherApplications)
	require.Equal(t, queryCount, provider.queryCalls)
	require.Equal(t, cancelCount, provider.cancelCalls)
	_, err = service.QuoteInvoice(ctx, owner.ID, InvoiceSelection{Selection: "all"})
	require.Equal(t, "INVOICE_UNPAID_EXISTS", infraerrors.Reason(err))
	_, err = service.CancelInvoice(ctx, other.ID, draft.ID)
	require.Equal(t, "INVOICE_NOT_FOUND", infraerrors.Reason(err))
	cancelled, err := service.CancelInvoice(ctx, owner.ID, draft.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceCancelled, cancelled.Status)
	_, err = service.CancelInvoice(ctx, owner.ID, draft.ID)
	require.NoError(t, err)
	noPayments, err := client.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(draft.ID)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, noPayments)
	require.Equal(t, queryCount, provider.queryCalls)
	require.Equal(t, cancelCount, provider.cancelCalls)

	unknown := newDraft("manual-unknown-payment")
	unknownFee := newFee(unknown)
	provider.resp = &payment.QueryOrderResponse{Status: payment.ProviderStatusPending, TradeNo: unknownFee.OutTradeNo}
	_, err = service.CancelInvoice(ctx, owner.ID, unknown.ID)
	require.Equal(t, "INVOICE_CANCEL_UNCONFIRMED", infraerrors.Reason(err))
	retained, err := service.GetInvoice(ctx, owner.ID, unknown.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceAwaitingPayment, retained.Status)
	provider.resp.Closed = true
	_, err = service.CancelOrder(ctx, unknownFee.ID, owner.ID)
	require.NoError(t, err, "cancellation from My Orders also releases the invoice after proven closure")
	retained, err = service.GetInvoice(ctx, owner.ID, unknown.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceCancelled, retained.Status)

	paidDuringCancel := newDraft("manual-paid-payment")
	paidFee := newFee(paidDuringCancel)
	provider.resp = &payment.QueryOrderResponse{Status: payment.ProviderStatusPaid, TradeNo: paidFee.OutTradeNo, Amount: 38}
	_, err = service.CancelInvoice(ctx, owner.ID, paidDuringCancel.ID)
	require.Equal(t, "INVOICE_ALREADY_PAID", infraerrors.Reason(err))
	paidApplication, err := service.GetInvoice(ctx, owner.ID, paidDuringCancel.ID)
	require.NoError(t, err)
	require.Equal(t, InvoicePending, paidApplication.Status)
	_, err = service.CancelInvoice(ctx, owner.ID, created.ID)
	require.Equal(t, "INVOICE_ALREADY_PAID", infraerrors.Reason(err))
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err, "migration replay preserves invoice history")
	paidOrders, err := client.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(created.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, paidOrders)
}
