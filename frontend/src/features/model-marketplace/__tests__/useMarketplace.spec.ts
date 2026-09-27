import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, reactive } from 'vue'
import { useMarketplace } from '../useMarketplace'
import { getModelPlaza } from '@/api/modelPlaza'
import {
  getMatrix,
  getSnapshot,
  type MonitorMatrixResponse,
  type MonitorSnapshot,
} from '@/api/channelMonitorV2'

vi.mock('@/api/modelPlaza', () => ({ getModelPlaza: vi.fn() }))
vi.mock('@/api/channelMonitorV2', () => ({ getMatrix: vi.fn(), getSnapshot: vi.fn() }))
const settings = reactive({
  publicSettingsLoaded: true,
  cachedPublicSettings: { channel_monitor_enabled: true, channel_monitor_mode: 'v2' },
  fetchPublicSettings: vi.fn(),
})
vi.mock('@/stores/app', () => ({ useAppStore: () => settings }))
const data = {
  items: [],
  coverage: { computed_at: '2026-09-01T12:00:00Z' },
} as unknown as MonitorMatrixResponse
const snapshot = { config: { refresh_interval_seconds: 60 }, coverage: {} } as MonitorSnapshot
let state: ReturnType<typeof useMarketplace>
let wrapper: ReturnType<typeof mount> | undefined
function setup() {
  wrapper = mount(
    defineComponent({
      setup() {
        state = useMarketplace()
        return () => null
      },
    }),
  )
}

describe('marketplace loading', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    settings.cachedPublicSettings.channel_monitor_mode = 'v2'
    vi.mocked(getModelPlaza).mockReset().mockResolvedValue({ groups: [], description: '' })
    vi.mocked(getMatrix).mockReset().mockResolvedValue(data)
    vi.mocked(getSnapshot).mockReset().mockResolvedValue(snapshot)
  })
  afterEach(() => {
    wrapper?.unmount()
    vi.useRealTimers()
  })

  it('keeps pricing available when monitoring fails and retries on the refresh interval', async () => {
    vi.mocked(getMatrix).mockRejectedValueOnce(new Error('failed'))
    vi.mocked(getSnapshot).mockRejectedValueOnce(new Error('failed'))
    setup()
    await flushPromises()
    expect(state.catalog.value).not.toBeNull()
    expect(state.monitorError.value).toBe(true)
    await vi.advanceTimersByTimeAsync(60000)
    await flushPromises()
    expect(state.monitorError.value).toBe(false)
    expect(state.matrix.value).toEqual(data)
  })
  it('uses matrix data even when the optional configuration request fails', async () => {
    vi.mocked(getSnapshot).mockRejectedValue(new Error('failed'))
    setup()
    await flushPromises()
    expect(state.matrix.value).toEqual(data)
    expect(state.monitorError.value).toBe(false)
  })
  it('does not call V2 endpoints in V1 mode', async () => {
    settings.cachedPublicSettings.channel_monitor_mode = 'v1'
    setup()
    await flushPromises()
    expect(getModelPlaza).toHaveBeenCalledOnce()
    expect(getMatrix).not.toHaveBeenCalled()
    expect(getSnapshot).not.toHaveBeenCalled()
  })
  it('aborts obsolete ranges and discards late responses', async () => {
    let finish!: (value: MonitorMatrixResponse) => void
    vi.mocked(getMatrix).mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve
      }),
    )
    setup()
    const signal = vi.mocked(getMatrix).mock.calls[0][3]
    state.range.value = '7d'
    await flushPromises()
    expect(signal?.aborted).toBe(true)
    expect(vi.mocked(getMatrix).mock.calls[1][0].range).toBe('7d')
    finish({ ...data, group_by: 'platform_model' })
    await flushPromises()
    expect(state.matrix.value).toEqual(data)
  })
  it('clears data when monitoring is disabled and stops polling after unmount', async () => {
    setup()
    await flushPromises()
    settings.cachedPublicSettings.channel_monitor_mode = 'v1'
    await flushPromises()
    expect(state.matrix.value).toBeNull()
    wrapper?.unmount()
    await vi.advanceTimersByTimeAsync(120000)
    expect(getMatrix).toHaveBeenCalledOnce()
  })
})
