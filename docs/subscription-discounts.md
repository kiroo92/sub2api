# Subscription checkout discounts

## 1. Scope

Subscription purchase discounts reuse the existing payment, order recovery, independent-subscription fulfillment and refund flows. They do not change subscription quotas or validity. Registration promo codes still grant balance and are a separate feature.

Admin entry: `/admin/orders/discount-codes`. Customers apply a code in the subscription confirmation screen on `/purchase`.

## 2. Signatures

- Migration `243_subscription_discount_codes.sql`: adds `subscription_discount_codes`; adds `discount_code_id`, `discount_state`, `discount_snapshot` to `payment_orders` with an indexed usage lookup and consistency check. Run Ent generation after schema changes.
- Admin `GET/POST /api/v1/admin/payment/discount-codes`, `PUT /api/v1/admin/payment/discount-codes/:id`: fixed code, `discount_type`, `discount_value`, `plan_ids`, `enabled`, nullable `expires_at`, `max_uses`, `per_user_limit`. PUT sends the complete configuration. Code names are immutable; disable codes instead of deleting history.
- Authenticated `POST /api/v1/payment/subscription-quote`: `{plan_id, payment_type, coupon_code?}`. Returns `original_amount`, `amount`, `pay_amount`, `fee_rate`, `currency`, optional `discount` snapshot. Preview does not reserve uses.
- Existing `POST /api/v1/payment/orders` accepts `coupon_code` and `expected_pay_amount` for subscription orders. A nonempty code requires the expected payable amount; it is only a stale-price check, never an authoritative price.
- Admin order listing supports `discount_code_id`; order responses expose `discount` and `discount_state`. Legacy anonymous trade-number lookup remains minimal.

## 3. Contracts

- `percentage` value means **percentage payable**: 80 = 八折 = pay 80%, not 80% off. The admin UI accepts a factor out of 10. `fixed_amount` subtracts in the plan's price units. Apply either to `price`, not the display-only `original_price`, using decimal rounding; then reuse existing conversion/fees/provider precision. No zero-price orders or implicit 0.01 minimum.
- Codes normalize to uppercase ASCII letters, digits, `_` and `-`, maximum 64 characters. `plan_ids=[]` means all plans; zero total/per-user limit means unlimited. Current codes may be disabled even after an originally selected plan has been deleted.
- One code can cover multiple selected plans and repeated purchases of the same plan. Per-user usage defaults to 1 and is shared across all eligible plans for that code; set it to 0 (or a higher limit) for repeated use. The total limit also aggregates every eligible plan, not a separate allowance per plan.
- The server validates and freezes the discount. `payment_orders.amount` is the discounted plan amount; `pay_amount` remains the actual gateway amount. Later plan/code changes do not reprice existing orders.
- `creating`, `reserved`, `consumed` count against total and per-user limits. Plan/code row locks serialize discounted order creation; no in-memory or Redis counter is the authority. Successful payment transitions the usage to `consumed` with PAID atomically, before the existing retryable entitlement delivery. A failed unpaid discounted order cannot be delivered via an admin retry.
- `released` requires proof that the provider was never called or its bound order is finally closed. Local CANCELLED/EXPIRED/FAILED and a nil cancellation error are insufficient. Provider `QueryOrderResponse.Closed` is distinct from failed: e.g. WeChat PAYERROR is not CLOSED.
- Unknown payment outcomes keep their reservation. The existing leader-locked payment reconciliation loop retries a bounded, rotating page. EasyPay currently cannot prove final closure, so unresolved uses may remain reserved; the admin list shows reservation counts and linked orders.
- Duplicate callbacks/release attempts are idempotent. Refunded orders keep consumed uses. Refunds use the existing stored order/pay-amount ratio and assigned subscription ID; they never refund the unpaid discount.
- Contradictory successful payment after proven closure/release is audited as `COUPON_PAYMENT_RECONCILIATION_REQUIRED` and blocks automatic delivery pending reconciliation. Do not silently discard the payment evidence or bypass limits.
- WeChat OAuth carries the code and expected amount in signed payment-resume claims; query values only help reconstruct the UI. A changed preview requires another user confirmation. Old unsigned/old-token no-discount flows remain compatible.
- Failed/stale preview never silently submits at full price. Editing code/plan/payment method invalidates it. Discounted mobile errors do not automatically create a second order, because the first provider call may already have succeeded and reserved the use.

## 4. Validation and errors

| Condition | Result |
| --- | --- |
| Unknown/disabled/expired code | `COUPON_NOT_FOUND` / `COUPON_DISABLED` / `COUPON_EXPIRED` |
| Code not valid for plan | `COUPON_PLAN_MISMATCH` |
| Used or reserved capacity exhausted | `COUPON_LIMIT_REACHED` / `COUPON_USER_LIMIT_REACHED` |
| Zero/negative discounted price, or rounding removes all savings | `COUPON_AMOUNT_INVALID` |
| Price changed or expected amount absent | `CHECKOUT_PRICE_CHANGED` |
| Coupon sent for recharge | `COUPON_SUBSCRIPTION_ONLY` |
| Unconfirmed discounted order retried for delivery | `PAYMENT_NOT_CONFIRMED` |
| Code rename or duplicate normalized name | `COUPON_CODE_IMMUTABLE` / `COUPON_DUPLICATE` |

## 5. Cases

- Base: no code follows the existing checkout unchanged.
- Good: price 100 with `VIP80` at percentage 80 yields 80; a fixed 15-off code yields 85. Entitlement duration/quota are unchanged.
- Good: with a 7 CNY conversion rate and 2% fee, discounted base 80 gives gateway amount 571.20. A full refund returns 571.20; a half-base refund returns 285.60.
- Bad: release capacity merely because a user closes checkout, then deliver a delayed successful payment without restoring accounting.

## 6. Verification

From `backend/`, run targeted `go test -p 2 -tags=unit ./internal/service ./internal/handler/... ./internal/server/... ./internal/payment/... -run 'Discount|Coupon|Payment|WeChat|Refund|MultiSubscription|AllSubscriptions|APIContracts' -count=1` and `go build ./cmd/server`.

`PAYMENT_DISCOUNT_TEST_DATABASE_URL` is exclusively for a disposable PostgreSQL database. `go test -tags=unit ./internal/service -run TestPaymentDiscountPostgres -count=1` creates/drops its own schema and verifies migration replay, concurrent limits and persistence constraints. SQLite alone does not validate PostgreSQL locks.

Frontend: `pnpm typecheck`, `pnpm lint:check`, related PaymentView/paymentFlow/paymentWechatResume/WechatPaymentCallbackView/AdminDiscountCodesView/refund/locale tests, then `pnpm build`. Component tests cover applying codes, stale previews, code errors and admin form semantics. Do not equate those checks with a real payment or browser visual check.

## 7. Wrong vs correct

Wrong: call the registration `PromoService.ApplyPromoCode` at checkout, trust a client discount amount, or use `original_price` for the calculation.

Correct: calculate in `payment_discount.go`, reserve through the existing order transaction, and charge/refund from the frozen order amounts. Use `PaymentOrderDiscount` to decode the saved snapshot once at the service boundary.

Deploy requires the new migration. After discounted orders exist, rollback must retain their payment/usage accounting; disable new codes before planning a compatible downgrade. Do not drop the new fields or start an old binary that can deliver unpaid discounted orders through generic retry.
