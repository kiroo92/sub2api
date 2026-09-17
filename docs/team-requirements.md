# Team billing requirements

Confirmed by the user on 2026-09-16; implementation authorized. Work on main only and preserve unrelated files. The user subsequently authorized committing and pushing the completed team feature. Deployment and migrations against live data remain outside this task.

## Outcome
One team owner funds members' independent team API keys using an ordered selection of the owner's available groups. Existing personal keys remain unchanged.

## Product contract
- Team keys support text only. Video, Live/realtime, and all image generation (synchronous, asynchronous, batch and image-generation tools) are excluded. This restriction does not disable personal-key features or ordinary text requests with image inputs.
- Platform administrators must configure the invitation domain/base URL before invitations can be sent. Invitation emails must contain a usable acceptance link; reuse the existing frontend URL setting and email-template system where compatible. Add a team-invitation template.
- Platform administrators list/search all teams by name, owner email and status; see/copy all full team keys, view members/usage/invitations/groups, edit name/groups/limits, remove members, revoke invitations, pause/resume, dissolve and retry billing. User confirmed these permissions. No ownership transfer or force-delete bypass for unsettled charges.
- Creation starts with a team name form. Overview contains members, email invitations (accept/resend/revoke), and usage. Keys and Settings are separate tabs.
- A user belongs to at most one team, including ownership. Members cannot create another team; owners cannot join another team. Enforce concurrent acceptance/creation in the database.
- Owner sees and copies all full team keys and all member usage without approval. Members create/manage/view/copy only their own team keys. Personal keys are outside this permission.
- Owner selects multiple available subscription/balance groups and orders them. Team keys share this order. Subscription groups consume owner's individual subscriptions in saved subscription order; balance groups consume owner's shared balance at that group's rates. Skip unavailable/expired/exhausted/model-incompatible sources before execution; never replay executed requests. No debit of member personal assets.
- Owner sets default and individual member daily/weekly/monthly money limits; zero means unlimited for that window. All keys aggregate per member. Any reached nonzero limit stops admission, never bypassed by group switching. Use actual billed cost and existing billing multipliers/window conventions; expose reset times. Owner's team keys count, default unlimited. Personal usage does not count. Leaving/rejoining does not reset current-window usage.
- Members may leave; owner may remove members. Their team keys immediately stop working. Owner cannot leave; no ownership transfer.
- Rename, pause/resume and dissolve. Pause blocks all team keys; resume restores eligible keys. Dissolution requires explicit confirmation, is irreversible, deletes team/membership/invitations/team keys/team usage/history after in-flight settlement. No retained historical team UI. Do not delete personal accounts or refund consumed funds.

## Acceptance
Verify owner/member permissions including full-key access, invitation ownership and races, one-team invariant, key revocation with cached auth, ordered owner-funded routing, shared per-member limits and rejoin persistence, unchanged personal billing, actual settlement attribution and dissolution cleanup. Verify frontend typecheck/lint/build and targeted backend tests, plus disposable-database checks when available.

## Exclusions
- Video, Live/realtime and image generation are explicitly unsupported and excluded by the user; do not track their integration as unfinished team work.
No new package/group-buy system, personal-key balance fallback, ownership transfer, approvals, live deployment or live database changes.
