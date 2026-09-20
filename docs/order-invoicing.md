# Whole-order invoicing

## 1. Scope and use

Users start from My Orders: **Invoice all** selects every eligible order across pages, while **Select orders** selects complete orders. Amounts are CNY; partial invoices and editable order amounts are not supported. Tax ID, buyer name and email are required; remarks are optional. The preview includes the service fee in the final invoice amount.

Opening the invoice page first checks for unpaid applications. If any exist, show them with a localized notice and a **Cancel application** button. Cancellation requires the user's confirmation; opening or refreshing the page never cancels, recreates or resumes an old payment. After cancellation succeeds, the user clicks **Apply again** to request a fresh quote. Truly empty eligibility is a normal empty state, not an English error message.

Administrators configure and process applications at `/admin/orders/invoices` under payment management. The feature defaults off. Set the item name, invoice tax rate and fee tiers, then enable applications. Administrators can filter, export selected/all filtered applications to Excel, and mark paid applications issued individually or in a batch. Export alone never changes invoice status. Email is exported for manual delivery; there is no automated issuance, email delivery, rejection, refund, upload/download or red-letter invoice workflow.

## 2. Interfaces and persistence

- Setting `payment_invoice_config`: `{enabled, item_name, tax_rate, tiers:[{upper_amount:number|null,type:"fixed"|"percentage",value:number}]}`. PUT replaces the complete configuration.
- User endpoints: `GET /api/v1/payment/invoices/config`, `POST /invoices/quote` with `{selection:"all"|"selected",order_ids?}`, `POST /invoices` with `{order_ids,tax_id,title,email,remarks,quote_fingerprint}` and `Idempotency-Key`, `GET /invoices/:id` for the owner.
- `GET /api/v1/payment/invoices/unpaid` is a read-only owned summary list; `POST /invoices/:id/cancel` performs explicit owned cancellation. User cancellation of an invoice fee through the existing payment-order endpoint uses the same invoice cancellation operation.
- Existing `POST /api/v1/payment/orders` supports `order_type:"invoice_fee"` and `invoice_request_id`. The fee comes from the saved application, never from a client-provided arbitrary amount.
- Admin endpoints under `/api/v1/admin/payment/invoices`: `GET/PUT /config`, `GET /`, `GET /:id`, `POST /mark-issued` with `{ids:[...]}`. Filters include status, search, user ID, start_date and end_date (UTC YYYY-MM-DD). Lists contain submitted/issued applications, not unpaid drafts.
- Migration `245_order_invoicing.sql` adds `invoice_requests`, `invoice_request_orders` and nullable unique `payment_orders.invoice_request_id`. A partial unique index on unreleased source order IDs prevents overlapping active applications. Monetary and invoice-order type checks are enforced in SQL. Ent code is generated using the existing `go generate ./ent` entry point.
- Authenticated order DTOs expose only the invoice summary `{id,status,total_amount}` and the fee order's request ID. Buyer details remain in owned/admin invoice endpoints; public order verification has no tax ID, email or remarks.

## 3. Amount and lifecycle contracts

### Prices

Source orders must belong to the user, have status COMPLETED, confirmed paid_at, no refund, positive CNY pay_amount, and type balance/subscription. `amount` is a credit or plan-price unit and is not the invoice base. Existing invoices and their service fees cannot be invoiced again.

For base B, select exactly one tier: lower bound exclusive, upper bound inclusive; the last tier has no upper bound. Fixed fee F is the configured value; a percentage fee is `round(B × percentage / 100, 2)`. There is no progressive accumulation and no surcharge applied to the surcharge. Decimal arithmetic determines amounts. Fees below CNY 0.01 after rounding are rejected rather than silently increased.

The tier fee is the exact payable service fee. It bypasses recharge multipliers, recharge min/max, recharge fee rate, subscription FX and coupon discounts. CNY provider availability, provider amount limits, pending-order limits and daily limits still apply.

Invoice gross G = B + F. For invoice tax rate r: net N = round(G / (1 + r / 100), 2), tax T = G - N. The tax rate is separate from the fee percentage and never increases G a second time. Configuration and eligibility are checked again against the quote fingerprint at application creation. Subsequent configuration changes do not reprice saved applications.

### Exactly one application/payment

Quote is read-only. CreateInvoice keeps an actor-scoped operation marker/fingerprint and atomically stores the request and its source order lines. Creation takes the invoice configuration row lock, rechecks the operation marker, and locks source orders in ascending ID order. It deliberately avoids a user row lock to prevent lock inversion with existing order/user refund transactions. This serializes new applications on a small configuration row; no Redis counter or extra scheduler is introduced.

New quotes/applications are blocked while the user has an outstanding unpaid application. A replay of the same application operation still returns its original record. The database check inside the creation lock prevents concurrent tabs from creating distinct unpaid applications after both observed an empty list.

The application state is `awaiting_payment → pending → issued`. `cancelled` is only unpaid payment cleanup, not an admin rejection flow. Creating the fee order locks the application and writes the unique payment binding before invoking a provider. Replays return the existing order identity/state; they never call the provider again. Backend fulfillment, direct balance/subscription entry points and refund preparation dispatch explicitly by order type.

Verified fee payments atomically complete their payment order and submit their application. The amount must match the saved fee exactly to the cent. Repeated/concurrent callbacks or administrator retries do not issue multiple invoices, credit balance, create redemption codes, issue subscriptions or grant affiliate rebates. Manual issued marking requires a completed, paid fee order, records the real admin actor/time, and is idempotent.

Local cancellation, expiry and create failure are not proof that the provider can no longer receive payment. Release source orders only if no provider call occurred, or its bound order is definitively Closed. A request with no payment order can expire safely. Unknown results retain their reservation and use the existing leader-locked payment reconciliation service. Channels without definitive closure evidence can remain reserved; never force-clear them merely because a local timeout elapsed. Contradictory payments after confirmed release are audited and rejected for automatic fulfillment.

### Recovery and compatibility

The invoice route reuses PaymentView, PaymentStatusPanel, paymentFlow and existing SDK pages. The deployed provider cannot reuse the same payment after its window closes, so the invoice page does not restore old payment snapshots and its status panel does not offer reopening old windows. An existing-payment response is marked `existing_payment:true`; the frontend shows the cancellation gate instead of relaunching it, including when its status is still PENDING.

The signed WeChat flow retains invoice_fee and the request ID. Automatic continuation is limited to the user's just-initiated OAuth handoff, recorded as a one-use tab session intent (`payment.invoice.oauth.intent`); stale returns without that intent only show the application gate. This intent is a UI guard, not authorization: the server still checks signed context and invoice ownership. Buyer data is not carried in URLs/OAuth tokens. Shared idempotency response storage redacts client_secret, so it is used for application creation only. Ordinary recharge/subscription recovery is unchanged.

Explicit cancellation may immediately release a draft with no payment binding. If a payment exists, use the bound provider's paid/closed evidence; confirmed paid applications cannot be cancelled, and an unknown closure retains reservations with a localized explanation. Successful cancellation clears only the matching invoice browser snapshot and never automatically creates another application or payment.

Disabling new invoicing does not prevent an existing application from completing payment or being marked issued. Original recharge/subscription refunds remain unchanged and do not rewrite invoice snapshots; invoice-fee refunds are explicitly unsupported. Do not roll back to an old binary that defaults unknown order types to balance while invoice_fee orders exist. Disable new applications for a controlled rollback instead of deleting records.

## 4. Validation and errors

| Condition | Result |
| --- | --- |
| Empty/foreign/unpaid/already reserved source selection | INVOICE_SELECTION_INVALID / INVOICE_ORDER_UNAVAILABLE / INVOICE_NO_ORDERS |
| Outstanding unpaid application | INVOICE_UNPAID_EXISTS; cancel explicitly before applying again |
| Paid application cancellation | INVOICE_ALREADY_PAID; no release |
| Cancellation without confirmed payment closure | INVOICE_CANCEL_UNCONFIRMED; retain reservation |
| Unsupported currency or inconsistent fee | INVOICE_CURRENCY_INVALID / INVOICE_AMOUNT_INVALID |
| Missing buyer fields or invalid email | INVOICE_INFO_INVALID |
| Invalid tiers or tax configuration | INVOICE_CONFIG_INVALID |
| Quote/configuration changed | INVOICE_QUOTE_CHANGED; review again |
| Reused operation key with different contents | IDEMPOTENCY_KEY_CONFLICT |
| Expired/cancelled request without an existing payment | INVOICE_NOT_PAYABLE |
| Existing payment creation | Return the existing payment, no new provider call |
| Unpaid application marked issued | INVOICE_NOT_SUBMITTED |
| Payment after confirmed release | INVOICE_PAYMENT_RECONCILIATION_REQUIRED |
| Invoice fee refund attempt | INVOICE_REFUND_UNSUPPORTED |

## 5. Examples

- Base: source payments CNY 311.50 plus fixed fee CNY 38 = invoice gross CNY 349.50. At 3% invoice tax, net is 339.32 and tax 10.18.
- Good: B=3000 with a 3% fee tier produces F=90 and G=3090; wallet balance is unchanged.
- Boundary: a `(0,500]` tier owns 500; 500.01 belongs to the next tier. Each application uses complete orders only.
- Bad: interpreting a recharge's USD credited `amount` as CNY, adding tax on top of already tax-inclusive G, reusing a service fee as a new invoice source, or releasing on local FAILED alone.

## 6. Verification

`go test -p 2 -tags=unit ./internal/service ./internal/handler/... ./internal/server/... ./internal/payment/... ./internal/repository -run 'Invoice|Payment|Discount|Coupon|WeChat|Refund|APIContracts' -count=1` and `go build ./cmd/server`.

For real PostgreSQL checks set `INVOICE_TEST_DATABASE_URL` exclusively to a disposable database, then run `go test -tags=unit ./internal/service -run '^TestInvoice' -count=1`. The test creates/drops its own schema and covers migration replay, 31-order selection, price drift, ownership, competing applications, duplicate fee creation, parallel notifications, zero balance/subscription/redemption effects, manual issuance, provider closure and release/payment races.

Frontend: typecheck, lint:check, InvoicePayment, AdminInvoicesView, invoiceExport, PaymentView, paymentFlow, paymentWechatResume, callback/result/status panel, UserOrdersView and locale tests, then build. Workbook tests round-trip a real XLSX to verify identifiers stay text and user strings are not formulas. Browser screenshot review is separate; do not equate component tests with visual QA.

The unpaid-cancellation regressions verify read-only entry, explicit confirmation, no automatic quote/create after cancellation, rejected stale WeChat/browser recovery, localized errors, and fresh authorized OAuth continuation. PostgreSQL checks cover no-payment cancellation, ownership, repetition, uncertain provider closure, paid-during-cancel and cancellation through My Orders. Run the complete backend `golangci-lint run --timeout=30m` using the CI workflow's version; frontend ESLint and Go test/build do not replace it.

## 7. Wrong vs correct

Wrong: treat every non-subscription payment as recharge; cache full SDK creation through the redacting idempotency helper; infer a closed provider payment from a local cancelled status.

Correct: explicit invoice_fee dispatch, unique persisted payment binding, preserved browser/WeChat context, and verified payment/closure evidence before changing invoice reservations.
