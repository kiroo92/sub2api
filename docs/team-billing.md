# Team billing implementation contract

## 1. Scope
Owner-funded team keys on main. Product decisions: [requirements](team-requirements.md); HTTP management contract: [API](team-api.md). Personal fixed-group/all-subscription behavior remains unchanged.

## 2. Signatures
- Migration `241_team_billing.sql`: teams, team_members, team_invitations, team_api_keys, team_requests; adds `team` routing mode with null persisted group.
- Migration `242_team_billing_recovery.sql`: closed/unresolved admission state and `team_billing_pending`, storing credential-free known-cost commands before debit. `POST /api/v1/team/billing/recover` is owner-only and accepts no amounts or request IDs from the client.
- `SubscriptionService.SelectForRequest` dispatches team mode, reads current membership/owner, selects configured groups, and records durable admission before execution.
- `UsageBillingCommand.TeamMemberID/TeamRequestID` retain membership across withdrawal and key revocation. Existing billing transaction applies owner debit and member counters with the same dedup claim.
- Team management endpoints use JWT `/api/v1/team`; do not accept payer/member attribution from request JSON.
- Admin list/config: `/api/v1/admin/teams` and `/api/v1/admin/teams/config`; ID-scoped details and mutations under `/api/v1/admin/teams/:team_id`. `AdminScope` requires the trusted administrator role, resolves owner from the team ID and adds `WithTeamAdminScope` without changing the authenticated actor. Repository `teamActor` checks the expected team ID under lock, preventing stale URLs from targeting a replacement team.
- Invitation sending requires `SettingKeyFrontendURL` from the settings table; the dedicated admin page edits that same system setting. `NotificationEmailEventTeamInvitation` (`team.invitation`) is listed in the existing template editor, with English/Chinese defaults and `team_name`, `invitation_url`, `expiry_time` plus common variables. Templates must retain `invitation_url`.

## 3. Contracts
- `TeamView.vue` uses the user's flat, compact reference style: pale page background, white thin-border panels, cyan actions/underline navigation, member identity rows and a separate invitation section with empty state. Member quota/usage and aggregate usage remain accessible through native expandable details. Styles are scoped to the team layout (also reused in administrator details); do not restyle global buttons/cards for this page. Preserve mobile, dark mode and keyboard controls.
- Persist API keys under owner user ID with separate team member association. Personal lists, ownership verification, edits and delete lookup exclude team keys. Team owner can copy all full team credentials; members only their own.
- Request-local `APIKey.Team`, owner, selected group/rate and exact subscription snapshots never enter credential cache. The durable `routing_mode=team` marker prevents stale deleted keys becoming personal keys. Check authoritative membership/key/team status on admission and WS turns.
- Group queue mixes owner subscriptions and permitted balance groups. Subscription records follow saved owner order inside a selected group. Skip expired/exhausted/incompatible/unavailable sources before execution. Do not replay completed requests. Group fallback fields are cleared on selected snapshots.
- Member money counters aggregate every team key and increment atomically with actual billed owner funds. Daily reset follows configured timezone midnight; week/month use 7/30-day windows. Zero means unlimited for that dimension; limits do not reserve unknown future token cost, so already admitted concurrent requests may finish beyond a limit.
- Active membership has a unique user constraint; owners also have membership rows. Membership and team row locks prevent removal/create-key races. Rejoining preserves existing counters/limits; defaults apply only to first join.
- Dissolution refuses outstanding durable requests (`TEAM_REQUESTS_PENDING`) and leaves state unchanged. Once settled, it deletes team-specific keys, usage, billing dedup, invitations, memberships, prompt/moderation/operational records and Redis image-task records, while preserving personal keys and already spent owner money. Returned/generated image files are not financial history and are not revoked from external object storage.
- Only text HTTP/WS requests are supported. Video, Live/realtime, image generation (including batch/async and generation tools) are explicitly excluded, not unfinished work. `ValidateTeamTextRequest` runs before team admission and each WS turn; rejects media endpoints, image models and structured output/tool capabilities, including duplicate JSON keys. Ordinary image inputs for text analysis remain allowed. Selected request-local groups disable image-generation permission; discovered image model IDs are filtered for team keys only. Personal keys remain unchanged.
- Known-cost debit failures preserve the command independently of the rolled-back debit. On request completion, mark admission closed; retain it until every pending command settles. Owner retry reuses the original dedup key, atomically debits funds, repairs the usage log's actual cost, and deletes the pending command. Recovery processes up to 100 commands per call; repeats and concurrent calls are idempotent. The member cannot start more requests while a pending bill or unresolved admission exists.
- Active execution contexts (including idle WebSockets) are never released by recovery. Crashes before close or failures before durable command capture remain unresolved and require operator reconciliation; no timeout-forgiveness or guessed usage. Closed, fully recorded requests can be recovered after restart. The owner sees pending request/bill counts and a Retry Settlement command. Members cannot see or invoke recovery. No polling/background recovery worker is introduced.
- Invalid/missing invitation frontend URL blocks send and resend before token creation. Do not build links from Host/forwarded headers. Links use `/team#invite=<random token>`; router moves the token to sessionStorage before redirecting to login so it does not enter login query strings. After login/registration, show the invitation and require explicit acceptance. The matching authenticated email, token expiry/revocation and one-team constraint remain authoritative. Resend rotates the token hash. No actual external invitation is sent by tests.
- Platform administrator permissions include all owner management except key creation on another user's behalf, self-leave and ownership transfer. Team key responses use `Cache-Control: no-store`. User routes cannot supply administrator scope. Same settlement/dissolution safeguards apply to admins.

## 4. Validation and errors
| Condition | Result |
|---|---|
| Existing membership during create/accept | `TEAM_CONFLICT` |
| Nonowner edits, foreign key/member, wrong invitation recipient | `TEAM_FORBIDDEN` / `TEAM_CONFLICT` |
| Paused/deleted team, removed member, disabled/revoked key | `TEAM_UNAVAILABLE` |
| Any reached nonzero member limit | `TEAM_MEMBER_LIMIT` |
| No eligible selected group | `SUBSCRIPTION_NOT_AVAILABLE` |
| Negative/nonfinite/excessive limits, invalid name | Validation error |
| Wrong dissolution name | `TEAM_CONFIRMATION_REQUIRED` |
| Pending request | `TEAM_REQUESTS_PENDING`; no partial deletion |
| Pending bill/unresolved member admission | `TEAM_REQUESTS_PENDING`; no further member admission |
| Command could not be persisted | `TEAM_BILLING_UNRECORDED`; retain unresolved admission |
| Missing/invalid configured invitation URL | `TEAM_INVITATION_DOMAIN_REQUIRED`; no new token/hash |
| Nonadministrator accesses admin team routes | 403 before domain access |
| Stale admin team ID resolves to different membership | `TEAM_FORBIDDEN`; no mutation |
| Image/video/Live request on team key | `TEAM_ENDPOINT_UNSUPPORTED` before upstream execution |

## 5. Cases
- Exhausted subscription A, available balance B: choose B and charge owner; member's personal balance is untouched.
- Zero owner balance but eligible subscription: request succeeds using exact subscription ID.
- Member leaves and rejoins: old keys remain revoked and current-period used amount persists.
- Failed database debit: keep durable request; never claim successful dissolution.
- Retry a recorded debit after database recovery: charge once, repair zero-cost error log, and release only the closed settled admission. Member requests to recover fail authorization.

## 6. Verification
- `TEAM_TEST_DATABASE_URL` must point only to a disposable PostgreSQL database. `go test ./internal/repository -run 'TestTeamPostgres|TestDeleteTeamImageTaskCache' -count=1` verifies concurrent create/accept/removal, scoped full keys, money quantization/dedup, late settlement, resets and deletion isolation.
- `go test -tags=unit ./internal/service ./internal/server/middleware ./internal/handler -run 'TestTeam|TestAllSubscriptions|TestApiKeyService_Delete' -count=1`; `go build ./cmd/server`; regenerate Wire with `go generate ./cmd/server` after provider changes.
- Frontend: `pnpm typecheck`, `pnpm lint:check`, `pnpm exec vitest run src/views/user/__tests__/TeamView.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts`, `pnpm build`.
- Recovery integration injects a database debit failure, asserts pending command survives rollback, retries failure without losing it, tests concurrent owner retries/late dedup, checks repaired usage cost, and ensures active/unknown requests cannot be forgiven. Service test serializes the pending snapshot to verify credentials are omitted. Frontend tests owner retry failure and hidden member controls.
- Admin checks cover scoped owner resolution without audit-actor impersonation, nonadmin denial, stale team isolation and no dissolution bypass. Invitation test uses local SMTP to check template rendering, escaped names, link/token hashes, rotation and domain gating. Text tests cover media HTTP, tools/duplicate keys/Gemini modalities/WS frames, preserving personal keys and image inputs. Frontend checks cover admin list/filter/config/detail, invite confirmation and login handoff; desktop/mobile mock-API browser checks verify layout and template preview.
- Full service suite currently includes baseline failure `TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort` (reproduced on pristine main). `TestRecordCyberPolicyEvent_RuntimeSnapshotRefreshFailureKeepsStaleScope` timed out in full suite but passed isolated. Do not represent full suite as green.

## 7. Wrong vs correct
Wrong: change `UsesAllSubscriptions` to include team balance; trust cached owner/group; delete keys before async billing; clear usage on rejoin.

Correct: distinct `UsesTeam` / `UsesDynamicRouting`; fresh authoritative team admission; exact payer/subscription snapshots; durable settlement tracking and transactional cleanup after completion.
