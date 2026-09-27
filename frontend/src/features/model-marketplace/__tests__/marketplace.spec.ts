import { describe, expect, it } from 'vitest'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import type {
  MonitorCoverage,
  MonitorHealth,
  MonitorMatrixBucket,
  MonitorMatrixRow,
  MonitorMetric,
} from '@/api/channelMonitorV2'
import {
  alignTimeline,
  availability,
  filterMarketplaceGroups,
  indexMonitorRows,
} from '../marketplace'

const groups = [
  {
    id: 1,
    name: 'Coding Pro',
    description: 'Fast coding',
    platform: 'openai',
    rate_multiplier: 1,
    user_rate_multiplier: 0.1,
    models: [{ name: 'gpt-test' }, { name: 'other' }],
  },
  {
    id: 2,
    name: 'Writer',
    description: 'Long context',
    platform: 'anthropic',
    rate_multiplier: 0.5,
    models: [{ name: 'claude-test' }],
  },
] as ModelPlazaGroup[]
const coverage = {
  requested_start: '2026-09-01T00:00:00Z',
  requested_end: '2026-09-01T00:20:00Z',
  data_through: '2026-09-01T00:15:00Z',
  bucket_seconds: 300,
} as MonitorCoverage

describe('model marketplace data', () => {
  it.each(['Coding', 'OpenAI', 'gpt-test', 'fast coding'])(
    'searches group/platform/model metadata: %s',
    (query) => {
      const result = filterMarketplaceGroups(groups, query, 'all', 'all')
      expect(result.map((group) => group.id)).toEqual([1])
      expect(result[0].models).toHaveLength(2)
    },
  )
  it('filters and sorts by the effective user rate', () => {
    expect(filterMarketplaceGroups(groups, '', 'all', 0.1).map((group) => group.id)).toEqual([1])
    expect(
      filterMarketplaceGroups(groups, '', 'anthropic', 'all').map((group) => group.id),
    ).toEqual([2])
    expect(filterMarketplaceGroups([...groups].reverse(), '', 'all', 'all')[0].id).toBe(1)
  })
  it('joins monitors by ID, keeping multiple platforms separate without guessing by name', () => {
    const rows = [
      { group_id: 1, group_name: 'Renamed', platform: 'openai' },
      { group_id: 1, platform: 'gemini' },
      { group_id: 99, group_name: 'Coding Pro', platform: 'openai' },
      { platform: 'openai' },
    ] as MonitorMatrixRow[]
    expect(indexMonitorRows(rows).get(1)).toEqual(rows.slice(0, 2))
    expect(indexMonitorRows(rows).get(2)).toBeUndefined()
  })
  it('uses retained health and rates when volume counters are redacted', () => {
    expect(
      availability(
        { request_count: 0, error_rate: 0.05 } as MonitorMetric,
        { overall: 'healthy', score: 95 } as MonitorHealth,
      ),
    ).toBe(0.95)
    expect(
      availability(
        { error_rate: 0 } as MonitorMetric,
        { overall: 'unknown', score: null } as MonitorHealth,
      ),
    ).toBeNull()
  })
  it('preserves interior and trailing gaps instead of shifting history toward now', () => {
    const buckets = [
      { bucket_start: '2026-09-01T00:00:00+00:00' },
      { bucket_start: '2026-09-01T00:10:00Z' },
    ] as MonitorMatrixBucket[]
    const slots = alignTimeline(buckets, coverage)
    expect(slots).toHaveLength(4)
    expect(slots.map((slot) => !!slot.bucket)).toEqual([true, false, true, false])
    expect(slots[3].end).toBe(Date.parse(coverage.requested_end!))
  })
  it('ignores out-of-range buckets and invalid coverage', () => {
    expect(
      alignTimeline(
        [{ bucket_start: '2026-08-31T23:55:00Z' }] as MonitorMatrixBucket[],
        coverage,
      ).every((slot) => !slot.bucket),
    ).toBe(true)
    expect(alignTimeline([], { ...coverage, bucket_seconds: 0 })).toEqual([])
  })
})
