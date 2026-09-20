# User dashboard and original subscriptions

## 1. Scope

The ordinary `/dashboard` combines account summaries with the original subscriptions. It does not change `/admin/dashboard`, profile, redemption, subscription billing, or the discontinued package/group-buy feature.

## 2. Entry points and signatures

- `views/user/DashboardView.vue` loads summary, wallet, quota and payment configuration independently; its child `components/user/dashboard/DashboardSubscriptions.vue` owns subscription reads, ordering and freeze/thaw.
- `UserDashboardStats.vue` renders four summary cards and expandable platform details. Its default slot places the subscription section between them, without another AppLayout.
- `/subscriptions` (including route name `Subscriptions`) redirects to `/dashboard#subscriptions`. There is no standalone user subscription page or sidebar item. Both user navigation and the administrator's personal navigation reach `/dashboard`; `/admin/subscriptions` remains the management page.
- `GET /api/v1/usage/dashboard/stats` adds optional numeric `today_subscription_cost` and `today_balance_cost`. The shared Go type uses `*float64` with `omitempty`; API-key summaries omit these fields because that path does not compute them.

## 3. Data and behavior contracts

- The four summaries are today's actual cost, available wallet balance, today's requests, and today's Token usage. Sum `actual_cost` by `billing_type` 1 (subscription) and 0 (balance), within the existing authenticated user/day aggregate. Show the split only when both fields exist. Do not infer success/failure counts from positive or zero charges.
- Token total uses `today_tokens`, the existing sum of input, output, cache creation and cache read tokens. Cache detail combines creation/read. A real zero is displayed as zero; a failed read is unavailable with retry.
- `user.balance` is already available balance; never subtract `frozen_balance` again. Recharge appears only outside simple mode, with public `payment_enabled === true` and payment config `enabled === true` plus `balance_disabled === false`. The payment config endpoint uses `enabled`, not `payment_enabled`.
- Stats, wallet and subscription failures do not hide each other's content/actions. Retries only reread the failed resource. Platform quotas remain in expandable details even if usage statistics fail.
- The subscription component mounts after public settings resolve and only outside simple mode with the subscription feature enabled. The existing subscription flag remains opt-out when unspecified. Disabled features are not re-enabled by compatibility redirects.
- Subscription cards form a responsive grid, numbered left-to-right then top-to-bottom. A desktop card has a maximum width of 360px. Expired/revoked history is collapsed and not reorderable.
- Only configured positive quota limits create rows; display `max(limit - used, 0) / limit`, with progress representing remaining quota. Use real platform/group data, acquired timestamps and authoritative reset timestamps. Frozen expiry is labeled as the expiry before the pause; frozen countdowns use `frozen_at` rather than wall time.
- Keep existing [freeze contracts](subscription-freeze.md), mutation locks, order rollback, cache invalidation and global-off thaw behavior. Frozen slots keep their position and are skipped during routing. All-subscriptions keys never fall back to wallet balance.

## 4. Error and compatibility matrix

| Situation | Result |
| --- | --- |
| Stats or wallet read fails | Unavailable/retry; subscription actions remain accessible |
| Subscription list fails | Error/retry, not an empty-subscriptions claim |
| One cost-split field missing | Keep actual total, omit incomplete split |
| Payment config fails or recharge disabled | Hide recharge action |
| Subscription feature disabled or simple mode | Do not mount/request the subscription section |
| Freeze globally disabled with an existing frozen row | Preserve thaw action |
| Order save fails | Restore saved order and explain failure |
| Old subscription URL/name | Authenticated dashboard, subscription anchor when visible |

## 5. Examples

- Base: a zero-usage day shows 0 tokens and requests, with $0 actual cost.
- Good: a frozen subscription with 3/10 used displays 7/10 remaining; advancing wall time does not change its paused validity or refresh time.
- Bad: a failed stats request hides subscriptions, or a missing billing split is rendered as $0 subscription spend.

## 6. Verification

From `frontend/`, run `pnpm typecheck`, `pnpm lint:check`, relevant Vitest tests and `pnpm build`. Tests cover the real DashboardView and subscription component, summary values/missing data, feature/payment switches, frozen clocks, ordering/rollback/history, sidebar and old URL/name routing.

From `backend/`, set `DASHBOARD_TEST_DATABASE_URL` to a disposable PostgreSQL database and run `go test -tags=unit ./internal/repository -run TestUserDashboardCostsPostgres -count=1`. It creates/drops an isolated schema, runs the real aggregate, and checks billing-source amounts, user isolation, the day boundary, free requests, zero usage, and omitted API-key fields. Never point preview binaries at application data just to inspect the UI.

Browser screenshots are a separate check; passing component tests does not establish desktop/mobile visual verification.

## 7. Wrong versus correct

Wrong: duplicate subscription state in a second page, render all dashboard content under `v-if="stats"`, use `actual_cost > 0` as request success, or call wallet availability a subscription fallback.

Correct: one subscription component, independent error states, truthful existing statistics, and the original subscription routing rules.
