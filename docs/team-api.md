# Team API contract

All endpoints use authenticated JWT and existing response envelope under `/api/v1`.

Platform administrator endpoints (trusted admin authentication, no approval workflow):
- GET `/admin/teams?search=&status=&page=1&page_size=20`: `{items:[{id,name,owner_id,owner_email,status,member_count,total_usage}],total,page,page_size}`; page size 1-100.
- GET `/admin/teams/activation-invitations`: pending, unexpired activation invitations (`id,email,expires_at,created_at`).
- POST `/admin/teams/activation-invitations` `{email}`: sends an administrator-only enterprise activation email and returns `{invitation_url}` for copying. Resending to the same email rotates the token. Requires configured frontend URL and SMTP; delivery failures are reported and the pending invitation remains available for resend/revoke.
- DELETE `/admin/teams/activation-invitations/:id`: revokes an unused activation invitation.
- GET/PUT `/admin/teams/config`: `{frontend_url:string}`; shared with the existing frontend URL setting. Empty disables invitation sending. URLs must be absolute HTTP(S), without credentials/query/fragment.
- `/admin/teams/:team_id` supports GET/PUT/DELETE and the same `/groups`, `/keys` (GET), `/keys/:id` (PUT/DELETE), `/members/:user_id/limits` (PUT), `/members/:user_id/models` (PUT), `/models` (GET), `/members/:user_id` (DELETE), `/invitations` (POST), `/invitations/:id/resend` (POST), `/invitations/:id` (DELETE), `/billing/recover` (POST) operations. Server resolves and pins the owning team; clients cannot choose payer IDs. No admin ownership transfer, member key creation or force-deletion.
- Team invitation templates reuse `/admin/settings/email-templates` and event `team.invitation` with `en`/`zh` locales. Enterprise activation uses the separate `team.activation` event and `/team#activate=<token>` link; the token survives login/registration and page refresh until creation succeeds. Accept member links through `/team#invite=<token>`, then POST `/team/invitations/accept` with the token after login and explicit confirmation.

- GET `/team`: `{team:Team|null,role:'owner'|'member'|'',members:Member[],invitations:Invitation[],pending_invitations:Invitation[],usage:Usage}`. Owner usage aggregates all members including former members; a member sees only own usage.
- POST `/team` `{name,activation_token}`: requires an administrator-issued, single-use, unexpired activation token bound to the authenticated account email; returns `403 TEAM_ACTIVATION_REQUIRED` for absent/invalid/expired/revoked/used tokens. Creation and token consumption are one transaction, serialized with membership acceptance. Existing teams retain access. After dissolution, creating another team requires a new invitation; PUT `/team` `{name?,status?:'active'|'paused',group_ids?:number[],default_limits?:Limits}`; DELETE `/team` `{confirm_name}`.
- POST `/team/leave`.
- POST `/team/billing/recover`: owner-only, no payload, returns `{recovered:number}`. Retries up to 100 closed-request bills; repeat when necessary. GET `/team` additionally reports `pending_requests` and retryable `pending_billing` counts to the owner (zero for members).
- POST `/team/invitations` `{email}`; POST `/team/invitations/:id/resend`; DELETE `/team/invitations/:id`; POST `/team/invitations/accept` `{token?,invitation_id?}` (authenticated recipient).
- PUT `/team/members/:user_id/limits` `{daily,weekly,monthly}`; DELETE `/team/members/:user_id`.
- GET `/team/models`: owner/admin-only, returns `string[]` from the team’s configured, active, owner-accessible groups using gateway model discovery (including composite routing and group allowlists), excluding unsupported generation models. Independent of individual member restrictions. When no groups are configured, previews models from all owner-accessible active groups so member restrictions can be set in advance; routing groups remain unchanged and requests still require configured pools. Team-key model discovery uses the same preview when no pools are configured, with each member’s allowlist applied to both ordinary and Codex catalogs.
- PUT `/team/members/:user_id/models` `{allowed_models:string[]}`: owner-only (also available to platform administrators under the team admin route). Sets an independent allowlist for this member, across all their team keys; `[]` means unrestricted. Required array, maximum 200 unique exact case-sensitive model IDs (each up to 200 bytes); no wildcards. Limits survive leaving/rejoining. Fresh state is checked before routing and again at admission; forbidden or missing request models return `403 TEAM_MODEL_FORBIDDEN` when restricted. Model discovery is filtered and WebSocket turns recheck permissions. Personal keys are unaffected.
- GET `/team/groups`: existing available group shape array, owner permitted groups.
- GET `/team/keys`: Key array; POST same `{name}`; PUT `/team/keys/:id` `{name?,status?:'active'|'disabled'}`; DELETE same.

Team: `{id,name,owner_id,status,group_ids:number[],default_limits:Limits,created_at}`.
Limits: `{daily:number,weekly:number,monthly:number}`.
Usage: `{daily:number,weekly:number,monthly:number,total:number}`.
Member: `{user_id,email,role,limits,allowed_models:string[],usage,resets:{daily,weekly,monthly},joined_at}`.
Invitation: `{id,team_id,team_name,email,status,expires_at,created_at}`.
Key: `{id,user_id,email,name,key,status,quota_used,created_at,last_used_at}`.

Owner sees all team keys, members see only own keys. Mutations refresh snapshot/list; no optimistic deletion on errors. Empty collections are arrays. Timestamps use JSON ISO format. Pause blocks request use but management remains available. Dissolution errors for in-flight work must preserve visible state and show actionable error, never claim deletion before cleanup.
