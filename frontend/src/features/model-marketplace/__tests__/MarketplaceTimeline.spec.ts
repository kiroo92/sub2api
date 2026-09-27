import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import MarketplaceTimeline from '../MarketplaceTimeline.vue'
import type { MonitorMatrixRow, MonitorCoverage } from '@/api/channelMonitorV2'
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }))
const metrics = { request_count: 0, error_rate: 0.05, cache_rate: 0.7, ttft: { p50_ms: 500 } }
const row = { platform: 'openai', group_id: 1, metrics, health: { overall: 'healthy', score: 95 }, buckets: [
  { bucket_start: '2026-09-01T00:05:00Z', metrics, health: { overall: 'healthy', score: 95 } },
] } as MonitorMatrixRow
const coverage = { requested_start: '2026-09-01T00:00:00Z', requested_end: '2026-09-01T00:10:00Z', bucket_seconds: 300 } as MonitorCoverage

describe('marketplace timeline', () => {
  it('renders redacted rates, missing history and accessible keyboard tooltips', async () => {
    const wrapper = mount(MarketplaceTimeline, { props: { row, coverage }, attachTo: document.body })
    try {
      expect(wrapper.get('strong').text()).toBe('95.00%')
      const bars = wrapper.findAll('button')
      expect(bars).toHaveLength(2)
      expect(bars[0].classes()).toContain('unknown')
      expect(bars[1].classes()).toContain('healthy')
      expect(bars[1].attributes('aria-label')).toContain('95.00%')
      await bars[0].trigger('keydown', { key: 'ArrowRight' })
      expect(document.activeElement).toBe(bars[1].element)
      expect(bars[1].attributes('tabindex')).toBe('0')
      expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('95.00%')
    } finally { wrapper.unmount() }
  })
})
