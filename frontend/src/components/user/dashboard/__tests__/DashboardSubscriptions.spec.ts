import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DashboardSubscriptions from '../DashboardSubscriptions.vue'
import type { UserSubscription } from '@/types'

const { list, reorder, showError, freeze, unfreeze, invalidate, settings } = vi.hoisted(() => ({ list: vi.fn(), reorder: vi.fn(), showError: vi.fn(), freeze: vi.fn(), unfreeze: vi.fn(), invalidate: vi.fn(), settings: { subscription_freeze_enabled: false } }))
vi.mock('@/api/subscriptions', () => ({ default: { getMySubscriptions: list, reorderSubscriptions: reorder, freezeSubscription: freeze, unfreezeSubscription: unfreeze } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, cachedPublicSettings: settings }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ invalidateCache: invalidate }) }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key }) }))

function subscription(id: number, status: UserSubscription['status'] = 'active'): UserSubscription {
  return {
    id, user_id: 7, group_id: id, status, sort_order: id,
    starts_at: '2020-01-01T00:00:00Z', expires_at: status === 'active' ? '2099-01-01T00:00:00Z' : '2020-01-02T00:00:00Z',
    daily_usage_usd: 0, weekly_usage_usd: 0, monthly_usage_usd: 0,
    daily_window_start: null, weekly_window_start: null, monthly_window_start: null,
    created_at: '', updated_at: ''
  }
}

describe('subscription order', () => {
  afterEach(() => vi.useRealTimers())
  beforeEach(() => {
    vi.clearAllMocks()
    settings.subscription_freeze_enabled = false
    list.mockResolvedValue([subscription(1), subscription(2), subscription(3, 'expired')])
    reorder.mockResolvedValue(undefined)
  })

  async function open() {
    const wrapper = mount(DashboardSubscriptions, { global: { stubs: { Icon: true, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } } })
    await flushPromises()
    return wrapper
  }

  it('saves active IDs only and removes renewal controls', async () => {
    const wrapper = await open()
    expect(wrapper.findAll('button[title="userSubscriptions.moveDown"]')).toHaveLength(2)
    expect(wrapper.text()).not.toContain('payment.renewNow')
    await wrapper.get('button[title="userSubscriptions.moveDown"]').trigger('click')
    await flushPromises()
    expect(reorder).toHaveBeenCalledWith([2, 1])
    expect(wrapper.findAll('h3').map((node) => node.text())).toEqual(['Group #2', 'Group #1'])
    expect(wrapper.findAll('[aria-label^="userSubscriptions.orderNumber"]').map(node => node.text())).toEqual(['01', '02'])
    await wrapper.get('button[aria-controls="subscription-history"]').trigger('click')
    expect(wrapper.get('#subscription-history').text()).toContain('Group #3')
    expect(wrapper.get('#subscription-history').find('button').exists()).toBe(false)
    wrapper.unmount()
  })

  it('locks controls during save and rolls back on failure', async () => {
    let fail!: (error: Error) => void
    reorder.mockImplementation(() => new Promise((_resolve, reject) => { fail = reject }))
    const wrapper = await open()
    await wrapper.get('button[title="userSubscriptions.moveDown"]').trigger('click')
    expect(wrapper.findAll('button[title], button.btn').every((node) => node.attributes('disabled') !== undefined)).toBe(true)
    fail(new Error('save failed'))
    await flushPromises()
    expect(wrapper.findAll('h3').map((node) => node.text())).toEqual(['Group #1', 'Group #2'])
    expect(showError).toHaveBeenCalledWith('userSubscriptions.failedToReorder')
    wrapper.unmount()
  })

  it('shows the frozen item and its thaw action when the global switch is off', async () => {
    const frozen = { ...subscription(1), frozen_at: '2026-09-20T20:00:00Z', expires_at: '2026-09-25T20:00:00Z', daily_usage_usd: 3, daily_resets_at: '2026-09-21T00:00:00Z', group: { name: 'Frozen plan', platform: 'openai', daily_limit_usd: 10 } }
    list.mockResolvedValue([frozen, subscription(2)])
    unfreeze.mockResolvedValue({ ...frozen, frozen_at: null, expires_at: '2099-01-01T00:00:00Z' })
    const wrapper = await open()
    expect(wrapper.text()).toContain('userSubscriptions.orderHint')
    expect(wrapper.text()).toContain('userSubscriptions.frozenHint')
    expect(wrapper.text()).toContain('$7.00 / $10.00')
    expect(wrapper.findAll('button').some(button => button.text() === 'userSubscriptions.freeze')).toBe(false)
    await wrapper.findAll('button').find(button => button.text() === 'userSubscriptions.unfreeze')!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === 'common.confirm')!.trigger('click')
    await flushPromises()
    expect(unfreeze).toHaveBeenCalledWith(1)
    expect(invalidate).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('shows freeze when enabled and includes frozen positions in ordering', async () => {
    settings.subscription_freeze_enabled = true
    list.mockResolvedValue([subscription(1), { ...subscription(2), frozen_at: '2026-09-20T20:00:00Z' }])
    const wrapper = await open()
    expect(wrapper.findAll('button').filter(b => b.text() === 'userSubscriptions.freeze')).toHaveLength(1)
    expect(wrapper.findAll('button').filter(b => b.text() === 'userSubscriptions.unfreeze')).toHaveLength(1)
    await wrapper.get('button[title="userSubscriptions.moveDown"]').trigger('click')
    await flushPromises()
    expect(reorder).toHaveBeenCalledWith([2, 1])
    expect(wrapper.text()).toContain('userSubscriptions.orderSaved')
    wrapper.unmount()
  })

  it('keeps frozen countdowns steady as wall time advances', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    vi.setSystemTime(new Date('2026-09-22T12:00:00Z'))
    list.mockResolvedValue([{ ...subscription(1), frozen_at: '2026-09-20T20:00:00Z', expires_at: '2026-09-25T20:00:00Z', daily_resets_at: '2026-09-21T00:00:00Z', group: { name: 'Frozen', daily_limit_usd: 10 }, daily_usage_usd: 3 }])
    const wrapper = await open()
    const before = wrapper.get('article').text()
    await vi.advanceTimersByTimeAsync(24 * 3600 * 1000)
    expect(wrapper.get('article').text()).toBe(before)
    wrapper.unmount()
  })
})
