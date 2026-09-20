# Subscription freezing and independent administrator grants

## 1. Scope

Original subscriptions on main. A user's pause is independent of the administrator status (`active`, `suspended`, revoked). Each subscription ID owns its validity, usage, position and freeze clock, including multiple subscriptions in the same group.

The “My Subscriptions” section of the user dashboard shows an ordered compact grid, explicit up/down actions with save feedback, and a freeze/thaw action at each card's bottom. The administrator setting controls new freezes only. Freezing never erases charges for work admitted before the pause. Dashboard and compatibility-link contracts: [user-dashboard.md](user-dashboard.md).

## 2. API and storage

- `subscription_freeze_enabled`: boolean DB setting, default false, exposed through admin/public settings. Omitted admin update fields preserve the previous value.
- `POST /api/v1/subscriptions/:id/freeze` and `POST /api/v1/subscriptions/:id/unfreeze`: owner-authenticated, no caller-supplied times or usage. Repeated freeze/thaw is idempotent.
- Migration `244_subscription_freeze.sql`: adds nullable `frozen_at`, nonnegative `frozen_duration_us`, nullable unique `admin_assignment_key` and `admin_assignment_fingerprint` to `user_subscriptions`.
- Subscription DTO additionally exposes `frozen_at`, `frozen_duration_us`, `remaining_seconds`, `daily_resets_at`, `weekly_resets_at`, `monthly_resets_at`, `is_one_time_daily_quota`. Private assignment keys/fingerprints are not exposed. While frozen, countdowns use frozen_at as their reference.
- Administrator `/subscriptions/assign` and `/subscriptions/bulk-assign` require `Idempotency-Key`. New operations create independent rows; replays of the same operation/target return the same row. Existing ID-specific extend/reset/revoke/restore remain explicit management actions.

## 3. Contracts

- Freeze only active, started, unexpired, owned records with the setting enabled. The server reads the setting; a stale visible button cannot grant permission. Thaw does not consult that setting or change administrator status. A revoked row cannot be thawed back into existence.
- Pause lifetime and daily/weekly/monthly refresh time together. Thaw atomically shifts expiry and activated window anchors by actual paused duration and accumulates that duration. It never restores an old usage snapshot over late billing or administrator adjustments.
- Daily midnight normalization runs in wall time minus accumulated pauses, then translates back. A subscription with four hours to refresh still has four hours after thaw; it is not forcibly aligned to the next wall-clock midnight. Never-frozen subscriptions keep existing midnight behavior. Weekly/monthly rolling periods retain their remaining duration.
- One-day, one-time daily quotas subtract accumulated pause from their term when classifying the product. Extending the wall-clock expiry must not turn them into repeatable daily quotas.
- Freeze/thaw lock owner then subscription, consistent with new-row append and reorder. Maintenance uses the current locked row and rejects frozen SQL reset/activation writes; a queued old request cannot overwrite a thawed clock. Freeze normalizes only already-due activated windows before recording the pause.
- Active discovery, expiration jobs, reminders and routing exclude frozen records. Fixed-group selection reads authoritative database state, preserving expiry-descending/ID ordering; it deliberately does not trust L1 subscription eligibility. This adds an indexed DB selection read to fixed-group admission. All-subscription/team routing uses each row's live freeze state. WebSocket new turns recheck the selection; in-flight work finishes against its admitted row.
- Reuse existing cache invalidation/billing attribution. Actual usage increments remain allowed after freezing, so freezing cannot waive an already admitted request's cost. No balance fallback is added.
- Frozen expiry adjustment uses the stopped reference time and preserves frozen state. Quota reset remains an administrator operation, using the stopped clock. Restoring a specific revoked subscription may coexist with another same-group subscription; it does not remove administrator suspension.
- Reorder includes active frozen slots. Legacy payloads containing the complete unfrozen set reorder only those slots. Reject duplicates/foreign/stale sets; never collide a frozen sort position with freshly compressed active positions. Frozen entries are skipped in use and return to their saved position on thaw.
- New explicit admin allocation uses the existing independent constructor, fresh validity/usage and appended order. Registration/default grants and redemption keep `AssignOrExtendSubscription`; internal calls without an operation key retain their legacy assignment behavior.
- Durable admin markers hash operation scope, authenticated actor and target user; the fingerprint includes group, normalized duration and notes. Markers are committed with the subscription, remain reserved after revocation, and protect replay even if the generic response record was not saved. A conflicting payload cannot reuse a marker. Batch duplicates are deduplicated; successful targets are not reissued on retry. Frontend uncertain requests retain their operation key in session storage; successful completion clears it so another intentional identical grant gets a new key.

## 4. Validation and errors

| Input/state | Outcome |
| --- | --- |
| New freeze while switch off/missing | `SUBSCRIPTION_FREEZE_DISABLED` |
| Frozen record selected for a new request | `SUBSCRIPTION_FROZEN` or no eligible subscription |
| Foreign/revoked ID | Not found/no mutation |
| Expired/suspended/not-yet-started new freeze | `SUBSCRIPTION_INVALID` |
| Clock precedes frozen time | `SUBSCRIPTION_CLOCK_CHANGED` |
| Pause overflows supported duration/expiry | `SUBSCRIPTION_TIME_LIMIT` |
| Admin assignment missing operation key | `IDEMPOTENCY_KEY_REQUIRED` |
| Same key/target with conflicting grant parameters | `IDEMPOTENCY_KEY_CONFLICT` |

## 5. Examples

- A has 5 days remaining, daily usage 3/10, reset in 4 hours. After a three-day freeze it still has 5 days and resets in 4 hours. If 2 units of previously admitted usage settle during the pause, thaw preserves 5/10 rather than restoring 3/10.
- With the switch turned off, A can still thaw, while B cannot begin a new freeze.
- Two administrator operations with the same user/group/days produce different IDs. Retrying one operation produces the same ID. Freezing the old ID does not freeze the new one.
- An administrator restoring a revoked ID never needs to delete another independent same-group subscription first.

## 6. Verification

Run targeted Go unit tests for Freeze/Subscription/Team/Payment refunds/settings/APIContracts, plus backend build. `SUBSCRIPTION_FREEZE_TEST_DATABASE_URL` must name a disposable PostgreSQL database; `go test ./internal/repository -run TestSubscriptionFreezePostgres -count=1` creates/drops its own schema and checks migration replay, independent/replayed grants, partial batches, frozen expiry, quota preservation, concurrent thaw, ordering and admin restrictions. Use `-tags=unit` for the broader handler/service suites.

Frontend: typecheck, lint:check, DashboardSubscriptions/DashboardView/settings/admin batch/API/operation-key/quota utility/locale tests, then build. Check frozen clocks while advancing mock time and the global-off thaw action. A passing component test is not a browser screenshot review.

## 7. Wrong versus correct

Wrong: set status=suspended for a user freeze, reset usage on thaw, run the old midnight correction on a shifted clock, or treat same user/group as an idempotency key for a new administrator grant.

Correct: keep frozen_at separate, shift the saved clocks atomically without overwriting usage, compute all quota displays from the same time helpers, and distinguish a new operation key from a replay.

Deployment applies an additive migration at startup. Do not start this build against application data just for a preview. Old binaries that ignore frozen_at are not safe while frozen records exist; rollback requires disabling new freezes and reconciling frozen records or retaining freeze-aware admission.
