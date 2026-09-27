import type { ModelPlazaGroup } from '@/api/modelPlaza'
import type {
  MonitorCoverage,
  MonitorHealth,
  MonitorMatrixBucket,
  MonitorMatrixRow,
  MonitorMetric,
} from '@/api/channelMonitorV2'
import { platformLabel } from '@/utils/platformColors'

export function effectiveGroupRate(group: ModelPlazaGroup): number {
  return group.user_rate_multiplier ?? group.rate_multiplier
}

export function marketplacePlatformPriority(platform: string): number {
  if (platform === 'openai') return 0
  if (platform === 'anthropic' || platform === 'claude') return 1
  return 2
}

export function filterMarketplaceGroups(
  groups: ModelPlazaGroup[],
  search: string,
  platform: string,
  rate: number | 'all',
) {
  const query = search.trim().toLocaleLowerCase()
  return groups
    .filter((group) => {
      if (platform !== 'all' && group.platform !== platform) return false
      if (rate !== 'all' && effectiveGroupRate(group) !== rate) return false
      return (
        !query ||
        [
          group.name,
          group.description,
          group.platform,
          platformLabel(group.platform),
          ...group.models.map((model) => model.name),
        ].some((value) => value.toLocaleLowerCase().includes(query))
      )
    })
    .sort((a, b) =>
      marketplacePlatformPriority(a.platform) - marketplacePlatformPriority(b.platform) ||
      effectiveGroupRate(a) - effectiveGroupRate(b) || a.id - b.id,
    )
}

export function indexMonitorRows(rows: MonitorMatrixRow[]) {
  const indexed = new Map<number, MonitorMatrixRow[]>()
  for (const row of rows) {
    if (row.group_id == null || row.group_id <= 0) continue
    const current = indexed.get(row.group_id) ?? []
    current.push(row)
    indexed.set(row.group_id, current)
  }
  return indexed
}

// Public monitoring payloads redact request counts. Use health, never the
// redacted count, to distinguish measured availability from missing samples.
export function availability(metrics: MonitorMetric, health: MonitorHealth): number | null {
  if (health.overall === 'unknown' && health.score == null) return null
  if (!Number.isFinite(metrics.error_rate)) return null
  return Math.max(0, Math.min(1, 1 - metrics.error_rate))
}

export interface TimelineSlot {
  start: number
  end: number
  bucket?: MonitorMatrixBucket
}
export function alignTimeline(
  buckets: MonitorMatrixBucket[],
  coverage: MonitorCoverage,
): TimelineSlot[] {
  const start = Date.parse(coverage.requested_start)
  const end = Date.parse(coverage.requested_end || coverage.data_through)
  const step = coverage.bucket_seconds * 1000
  if (![start, end, step].every(Number.isFinite) || step <= 0 || start >= end) return []
  const first = Math.floor(start / step) * step
  const count = Math.ceil((end - first) / step)
  if (count > 2000) return []
  const byStart = new Map(buckets.map((bucket) => [Date.parse(bucket.bucket_start), bucket]))
  return Array.from({ length: count }, (_, index) => {
    const time = first + index * step
    return { start: time, end: Math.min(time + step, end), bucket: byStart.get(time) }
  })
}
