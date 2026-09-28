package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequest"
	"github.com/Wei-Shaw/sub2api/ent/invoicerequestorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
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
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDIn(ids...)).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	byOrder := make(map[int64]*InvoiceResult, len(orders))
	orderIDs := make([]int64, 0, len(orders))
	byInvoice := make(map[int64]*InvoiceResult, len(result))
	for i := range result {
		byInvoice[result[i].ID] = &result[i]
	}
	for _, order := range orders {
		if order.InvoiceRequestID != nil {
			byOrder[order.ID] = byInvoice[*order.InvoiceRequestID]
			orderIDs = append(orderIDs, order.ID)
		}
	}
	if err := s.loadInvoiceDelivery(ctx, orderIDs, byOrder); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// loadInvoiceDelivery only reads delivery metadata; invoice files are never returned in the API.
func (s *PaymentService) loadInvoiceDelivery(ctx context.Context, orderIDs []int64, results map[int64]*InvoiceResult) error {
	if len(orderIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(orderIDs))
	for _, id := range orderIDs {
		keys = append(keys, strconv.FormatInt(id, 10))
	}
	logs, err := s.entClient.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDIn(keys...), paymentauditlog.ActionEQ("INVOICE_EMAIL_SENT")).All(ctx)
	if err != nil {
		return err
	}
	for _, log := range logs {
		id, err := strconv.ParseInt(log.OrderID, 10, 64)
		if err != nil || results[id] == nil {
			continue
		}
		result := results[id]
		if result.DeliveredAt != nil && result.DeliveredAt.After(log.CreatedAt) {
			continue
		}
		at := log.CreatedAt
		result.DeliveredAt = &at
		var detail struct {
			Filename string `json:"filename"`
		}
		if json.Unmarshal([]byte(log.Detail), &detail) == nil {
			result.AttachmentName = detail.Filename
		}
	}
	return nil
}

const maxInvoiceAttachmentBytes = 10 << 20

// SendInvoiceAttachment sends the paid invoice PDF to the email recorded on the application.
func (s *PaymentService) SendInvoiceAttachment(ctx context.Context, actor, invoiceID int64, attachment []byte) (*InvoiceResult, error) {
	if actor <= 0 || invoiceID <= 0 {
		return nil, infraerrors.BadRequest("INVOICE_SELECTION_INVALID", "invalid invoice selection")
	}
	if len(attachment) < 5 || len(attachment) > maxInvoiceAttachmentBytes || !bytes.HasPrefix(attachment, []byte("%PDF-")) {
		return nil, infraerrors.BadRequest("INVOICE_ATTACHMENT_INVALID", "upload a PDF invoice up to 10 MB")
	}
	if s.notificationEmailService == nil || s.notificationEmailService.emailService == nil {
		return nil, ErrEmailNotConfigured
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := tx.InvoiceRequest.Query().Where(invoicerequest.IDEQ(invoiceID))
	if paymentAuditDialect(tx.Client()) == dialect.Postgres {
		q.ForUpdate()
	}
	row, err := q.Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.NotFound("INVOICE_NOT_FOUND", "invoice application not found")
	}
	if err != nil {
		return nil, err
	}
	if row.SubmittedAt == nil || (row.Status != InvoicePending && row.Status != InvoiceIssued) {
		return nil, infraerrors.Conflict("INVOICE_NOT_SUBMITTED", "only paid submitted invoices can be delivered")
	}
	order, err := tx.PaymentOrder.Query().Where(paymentorder.InvoiceRequestIDEQ(invoiceID), paymentorder.StatusEQ(OrderStatusCompleted), paymentorder.PaidAtNotNil()).Only(ctx)
	if err != nil {
		return nil, infraerrors.Conflict("INVOICE_NOT_SUBMITTED", "invoice fee is not settled")
	}
	orderID := strconv.FormatInt(order.ID, 10)
	alreadySent, err := tx.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(orderID), paymentauditlog.ActionEQ("INVOICE_EMAIL_SENT")).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if alreadySent {
		return nil, infraerrors.Conflict("INVOICE_ALREADY_DELIVERED", "invoice email has already been sent")
	}
	filename := fmt.Sprintf("invoice-%d.pdf", invoiceID)
	subject := "【OPEN1 CODE】发票已开具"
	body := "<p>您好：</p>" +
		"<p>您的发票已开具，PDF 文件见本邮件附件，请查收。</p>" +
		"<p>如发票信息有误，或需要协助处理其他问题，请直接回复本邮件联系我们。</p>" +
		"<p>OPEN1 CODE 团队</p>"
	if err := s.notificationEmailService.emailService.SendEmailWithAttachment(ctx, row.Email, subject, body, filename, attachment); err != nil {
		return nil, err
	}
	if row.Status == InvoicePending {
		if _, err := tx.InvoiceRequest.UpdateOneID(invoiceID).SetStatus(InvoiceIssued).SetIssuedAt(time.Now()).SetIssuedBy(actor).Save(ctx); err != nil {
			return nil, err
		}
		if _, err := tx.PaymentAuditLog.Create().SetOrderID(orderID).SetAction("INVOICE_ISSUED").SetOperator("admin:" + strconv.FormatInt(actor, 10)).SetDetail("{}").Save(ctx); err != nil {
			return nil, err
		}
	}
	detail, _ := json.Marshal(map[string]any{"invoice_id": invoiceID, "filename": filename})
	if _, err := tx.PaymentAuditLog.Create().SetOrderID(orderID).SetAction("INVOICE_EMAIL_SENT").SetOperator("admin:" + strconv.FormatInt(actor, 10)).SetDetail(string(detail)).Save(ctx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetInvoice(ctx, 0, invoiceID)
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
