# Scheduler Outbox Remediation Note

Date: 2026-04-08

## Problem Summary

- `scheduler_outbox` backlog is very large.
- Redis watermark `sched:outbox:watermark` is stuck near `93400`.
- Unconsumed events started around `2026-03-01 15:11:52 +08`.
- The outbox worker frequently logs:
  - `outbox watermark write failed: context deadline exceeded`
  - `outbox lag warning`
  - `outbox lag rebuild triggered`
  - `outbox backlog rebuild triggered`

## Current Root Cause Assessment

- The worker uses one shared timeout for the whole poll batch.
- A batch can contain many expensive `account_changed` events.
- Event handling can consume most or all of the batch deadline.
- Watermark persistence happens at the end of the batch.
- When the deadline expires before watermark write, the same batch is retried repeatedly.
- Rebuild-on-lag and rebuild-on-backlog can amplify load and make progress even harder.

## Immediate Operational Mitigation

1. Disable or relax automatic lag/backlog rebuild triggers.
2. Back up Redis watermark and database state.
3. Advance watermark to current max outbox ID if a cache rebuild will be performed.
4. Trigger one full scheduler snapshot rebuild from current database state.
5. Observe whether watermark starts moving forward normally.

## Code Changes To Implement

1. Do not use one short shared timeout for the full outbox batch.
2. Separate event-processing budget from watermark-write budget.
3. Persist progress incrementally so processed events are not replayed forever.
4. Reduce redundant rebuild work by coalescing repeated account/group events within one batch.
5. Keep `account_last_used` fast-path lightweight.
6. Prevent lag/backlog rebuild loops from repeatedly adding pressure when the system is already behind.
7. Lower noisy warning logs that represent expected cache misses rather than real failures.

## Success Criteria

- Watermark advances continuously.
- Outbox backlog shrinks over time.
- Repeated `outbox watermark write failed` logs stop.
- Lag and backlog rebuild logs become rare.
- Request-side scheduler errors caused by stale cache become less frequent.
