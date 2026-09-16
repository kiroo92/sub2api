import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubscriptionsView from '../SubscriptionsView.vue'
import type { UserSubscription } from '@/types'

const { list, reorder, showError } = vi.hoisted(() => ({ list: vi.fn(), reorder: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/subscriptions', () => ({ default: { getMySubscriptions: list, reorderSubscriptions: reorder } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

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
  beforeEach(() => {
    vi.clearAllMocks()
    list.mockResolvedValue([subscription(1), subscription(2), subscription(3, 'expired')])
    reorder.mockResolvedValue(undefined)
  })

  async function open() {
    const wrapper = mount(SubscriptionsView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } } })
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
    expect(wrapper.findAll('h3').map((node) => node.text())).toEqual(['Group #2', 'Group #1', 'Group #3'])
    wrapper.unmount()
  })

  it('locks controls during save and rolls back on failure', async () => {
    let fail!: (error: Error) => void
    reorder.mockImplementation(() => new Promise((_resolve, reject) => { fail = reject }))
    const wrapper = await open()
    await wrapper.get('button[title="userSubscriptions.moveDown"]').trigger('click')
    expect(wrapper.findAll('button').every((node) => node.attributes('disabled') !== undefined)).toBe(true)
    fail(new Error('save failed'))
    await flushPromises()
    expect(wrapper.findAll('h3').map((node) => node.text())).toEqual(['Group #1', 'Group #2', 'Group #3'])
    expect(showError).toHaveBeenCalledWith('userSubscriptions.failedToReorder')
    wrapper.unmount()
  })
})
