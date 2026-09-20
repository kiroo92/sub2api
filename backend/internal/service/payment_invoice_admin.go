package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequest"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequestorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type InvoiceListParams struct {
	Page      int
	PageSize  int
	Status    string
	Search    string
	UserID    int64
	StartDate string
	EndDate   string
}

func (s *PaymentService) ListInvoices(ctx context.Context, p InvoiceListParams) ([]InvoiceResult, int, error) {
	size, page := applyPagination(p.PageSize, p.Page)
	q := s.entClient.InvoiceRequest.Query().Where(invoicerequest.StatusIn(InvoicePending, InvoiceIssued))
	if p.Status != "" {
		if p.Status != InvoicePending && p.Status != InvoiceIssued {
			return nil, 0, infraerrors.BadRequest("INVOICE_STATUS_INVALID", "invalid invoice status")
		}
		q.Where(invoicerequest.StatusEQ(p.Status))
	}
	if p.UserID > 0 {
		q.Where(invoicerequest.UserIDEQ(p.UserID))
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		id, _ := strconv.ParseInt(search, 10, 64)
		q.Where(invoicerequest.Or(invoicerequest.IDEQ(id), invoicerequest.TitleContainsFold(search), invoicerequest.TaxIDContainsFold(search), invoicerequest.EmailContainsFold(search)))
	}
	for _, filter := range []struct {
		raw string
		end bool
	}{{p.StartDate, false}, {p.EndDate, true}} {
		if filter.raw == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", filter.raw)
		if err != nil {
			return nil, 0, infraerrors.BadRequest("INVOICE_DATE_INVALID", "use YYYY-MM-DD dates")
		}
		if filter.end {
			q.Where(invoicerequest.SubmittedAtLT(date.AddDate(0, 0, 1)))
		} else {
			q.Where(invoicerequest.SubmittedAtGTE(date))
		}
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.Order(dbent.Desc(invoicerequest.FieldID)).Limit(size).Offset((page - 1) * size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	ids := []int64{}
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	result := []InvoiceResult{}
	if len(ids) == 0 {
		return result, total, nil
	}
	items, err := s.entClient.InvoiceRequestOrder.Query().Where(invoicerequestorder.InvoiceRequestIDIn(ids...)).Order(dbent.Asc(invoicerequestorder.FieldOrderID)).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	grouped := map[int64][]*dbent.InvoiceRequestOrder{}
	for _, item := range items {
		grouped[item.InvoiceRequestID] = append(grouped[item.InvoiceRequestID], item)
	}
	for _, row := range rows {
		result = append(result, invoiceResult(row, grouped[row.ID]))
	}
	return result, total, nil
}

func (s *PaymentService) MarkInvoicesIssued(ctx context.Context, actor int64, ids []int64) error {
	ids, err := normalizeInvoiceIDs(ids)
	if err != nil {
		return err
	}
	if actor <= 0 || len(ids) > 1000 {
		return infraerrors.BadRequest("INVOICE_SELECTION_INVALID", "invalid invoice selection")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := tx.InvoiceRequest.Query().Where(invoicerequest.IDIn(ids...)).Order(dbent.Asc(invoicerequest.FieldID))
	if paymentAuditDialect(tx.Client()) == dialect.Postgres {
		q.ForUpdate()
	}
	rows, err := q.All(ctx)
	if err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return infraerrors.NotFound("INVOICE_NOT_FOUND", "one or more invoice applications do not exist")
	}
	for _, row := range rows {
		if row.SubmittedAt == nil || (row.Status != InvoicePending && row.Status != InvoiceIssued) {
			return infraerrors.Conflict("INVOICE_NOT_SUBMITTED", "only paid submitted invoices can be marked issued")
		}
	}
	for _, row := range rows {
		if row.Status == InvoiceIssued {
			continue
		}
		order, err := tx.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(row.ID), paymentorder.StatusEQ(OrderStatusCompleted), paymentorder.PaidAtNotNil()).Only(ctx)
		if err != nil {
			return infraerrors.Conflict("INVOICE_NOT_SUBMITTED", "invoice fee is not settled")
		}
		if _, err := tx.InvoiceRequest.UpdateOneID(row.ID).SetStatus(InvoiceIssued).SetIssuedAt(time.Now()).SetIssuedBy(actor).Save(ctx); err != nil {
			return err
		}
		if _, err := tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(order.ID, 10)).SetAction("INVOICE_ISSUED").SetOperator("admin:" + strconv.FormatInt(actor, 10)).SetDetail("{}").Save(ctx); err != nil {
			return err
		}
	}
	return tx.Commit()
}
