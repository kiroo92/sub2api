import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import DashboardView from '../DashboardView.vue'
import RedeemView from '../RedeemView.vue'
import SubscriptionsView from '../SubscriptionsView.vue'
import OwnedPackages from '@/components/packages/OwnedPackages.vue'
import type { UserPackage } from '@/types/packages'
import type { UserDashboardStats } from '@/api/usage'
import en from '@/i18n/locales/en'
import { formatDateTimeToMinute } from '@/utils/format'

const mocks = vi.hoisted(() => ({
  stats: vi.fn(), trend: vi.fn(), models: vi.fn(), usage: vi.fn(), refreshUser: vi.fn(),
  mine: vi.fn(), reorder: vi.fn(), legacy: vi.fn(), redeem: vi.fn(), history: vi.fn(),
  activeSubscriptions: vi.fn(), quotas: vi.fn(), config: vi.fn(), success: vi.fn(), error: vi.fn(),
  auth: {} as { user: { balance: number; frozen_balance: number; concurrency: number } | null; isSimpleMode: boolean },
  app: {} as { cachedPublicSettings: { payment_enabled: boolean }; showSuccess: ReturnType<typeof vi.fn>; showError: ReturnType<typeof vi.fn> }
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ ...mocks.auth, refreshUser: mocks.refreshUser }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks.app }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ fetchActiveSubscriptions: mocks.activeSubscriptions }) }))
vi.mock('@/api/usage', () => ({ usageAPI: { getDashboardStats: mocks.stats, getDashboardTrend: mocks.trend, getDashboardModels: mocks.models, getByDateRange: mocks.usage } }))
vi.mock('@/api/user', () => ({ getMyPlatformQuotas: mocks.quotas }))
vi.mock('@/api/payment', () => ({ paymentAPI: { getConfig: mocks.config } }))
vi.mock('@/api/packages', () => ({ packagesAPI: { mine: mocks.mine, reorder: mocks.reorder } }))
vi.mock('@/api/subscriptions', () => ({ default: { getMySubscriptions: mocks.legacy } }))
vi.mock('@/api/redeem', () => ({ redeemAPI: { redeem: mocks.redeem, getHistory: mocks.history } }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string, params: Record<string, string | number> = {}) => {
  const message = key.split('.').reduce<unknown>((value, part) => value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined, en)
  return typeof message === 'string' ? message.replace(/\{(\w+)\}/g, (_, param: string) => String(params[param] ?? '')) : key
} }) }))

const stats = { today_actual_cost: 1.25, today_cost: 2.5, today_requests: 12, total_requests: 123, active_api_keys: 2, total_api_keys: 3 } as UserDashboardStats
function holding(id: number, expired = false): UserPackage {
  const period = { id: id * 10, package_id: id, period_index: 1, starts_at: '2026-09-13T00:00:00Z', ends_at: '2026-09-20T00:00:00Z', quota_usd: 100, used_usd: id * 10 }
  return {
    id, user_id: 1, group_id: 1, order_id: id, status: 'active', sort_order: id, group_buy_id: 4,
    starts_at: period.starts_at, expires_at: expired ? '2026-09-13T00:00:00Z' : period.ends_at, current_period: period, periods: [period],
    plan: { id: 1, name: `Card ${id}`, group_id: 1, group_name: 'OpenAI', group_platform: 'openai', description: '', validity_days: 7, base_quota_usd: 100, price: 10, currency: 'USD', group_buy_enabled: true, group_buy_hours: 24, tiers: [], for_sale: true, sort_order: 1 }
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
const wrappers: ReturnType<typeof mount>[] = []
function render(component = DashboardView) {
  const wrapper = mount(component, { global: { stubs: {
    AppLayout: { name: 'AppLayout', template: '<main><slot /></main>' },
    RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
    VueDraggable: { name: 'VueDraggable', props: ['modelValue', 'disabled'], template: '<div data-testid="sortable"><slot /></div>' }
  } } })
  wrappers.push(wrapper)
  return wrapper
}
const order = (wrapper: ReturnType<typeof mount>) => wrapper.findAll('[data-testid="sortable"] [data-package-id]').map(card => card.attributes('data-package-id'))

describe('ordinary dashboard account overview', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-14T00:00:00Z'))
    mocks.auth = reactive({ user: { balance: 18, frozen_balance: 7, concurrency: 3 }, isSimpleMode: false })
    mocks.app = reactive({ cachedPublicSettings: { payment_enabled: true }, showSuccess: mocks.success, showError: mocks.error })
    mocks.stats.mockReset().mockResolvedValue(stats)
    mocks.refreshUser.mockReset().mockResolvedValue(undefined)
    mocks.mine.mockReset().mockResolvedValue([holding(1), holding(2), holding(3, true)])
    mocks.reorder.mockReset().mockResolvedValue(undefined)
    mocks.legacy.mockReset().mockResolvedValue([{ id: 1, group_id: 1, status: 'active', expires_at: '2026-10-01T00:00:00Z', daily_usage_usd: 2, group: { name: 'Legacy card', platform: 'openai', daily_limit_usd: 10 } }])
    mocks.history.mockReset().mockResolvedValue([])
    mocks.redeem.mockReset().mockResolvedValue({ message: 'Applied', type: 'balance', value: 10, new_balance: 28 })
    mocks.activeSubscriptions.mockReset().mockResolvedValue([])
    mocks.quotas.mockReset().mockResolvedValue({ platform_quotas: [] })
    mocks.config.mockReset().mockResolvedValue({ data: { enabled: true, balance_disabled: false } })
  })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })

  it('shows four true summary metrics, ordered holdings and inline redeem without usage charts', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('[data-testid="dashboard-summary"] article')).toHaveLength(4)
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('$18.00')
    expect(wrapper.text()).not.toContain('$11.00')
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('$1.25')
    expect(wrapper.get('a[href="/purchase?tab=recharge"]').text()).toBe('Balance Top-Up')
    expect(wrapper.get('#subscriptions').text()).toContain('Legacy package')
    expect(wrapper.get('#subscriptions').text()).toContain('$2.00 / $10.00')
    expect(wrapper.get('#subscriptions').text()).toContain('$90.00')
    expect(wrapper.findAll('[data-legacy-id] button')).toHaveLength(0)
    expect(order(wrapper)).toEqual(['1', '2'])
    expect(wrapper.findAll('details [data-package-id]')).toHaveLength(1)
    expect(wrapper.find('a[href="/package-groups/4"]').exists()).toBe(true)
    expect(wrapper.get('#subscriptions').element.nextElementSibling).toBe(wrapper.get('#redeem').element)
    expect(mocks.trend).not.toHaveBeenCalled()
    expect(mocks.models).not.toHaveBeenCalled()
    expect(mocks.usage).not.toHaveBeenCalled()
  })

  it('keeps loaded holdings when summary fails and retries without zeroing missing stats', async () => {
    mocks.stats.mockRejectedValueOnce(new Error('offline'))
    mocks.quotas.mockRejectedValueOnce(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('--')
    expect(wrapper.get('[role="alert"]').text()).toContain('Could not load usage overview')
    expect(order(wrapper)).toEqual(['1', '2'])
    await wrapper.get('button[aria-label="Refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('$1.25')
  })

  it.each(['active', 'expired', 'suspended'])('shows %s legacy-only holdings with original quotas and no ordering controls', async status => {
    mocks.mine.mockResolvedValue([])
    mocks.legacy.mockResolvedValue([{
      id: 7, group_id: 1, status,
      expires_at: status === 'expired' ? '2026-09-01T00:00:00Z' : '2026-10-01T00:00:00Z',
      daily_usage_usd: 2, weekly_usage_usd: 14, monthly_usage_usd: 40,
      group: { name: 'Legacy card', platform: 'openai', daily_limit_usd: 10, weekly_limit_usd: 70, monthly_limit_usd: 300 }
    }])
    const wrapper = render()
    await flushPromises()
    const subscriptions = wrapper.get('#subscriptions')
    const card = subscriptions.get('[data-legacy-id="7"]')
    expect(card.text()).toContain('Legacy card')
    expect(card.text()).toContain('Legacy package')
    expect(card.text()).toContain('$2.00 / $10.00')
    expect(card.text()).toContain('$14.00 / $70.00')
    expect(card.text()).toContain('$40.00 / $300.00')
    expect(card.text()).toContain(status === 'expired' ? 'Expired' : formatDateTimeToMinute(new Date('2026-10-01T00:00:00Z')))
    expect(subscriptions.text()).not.toContain('No subscription packages yet')
    expect(subscriptions.text()).not.toContain('No new packages yet')
    expect(subscriptions.find('button[aria-label^="Move "], .package-drag').exists()).toBe(false)
    expect(mocks.reorder).not.toHaveBeenCalled()
    if (status === 'active') {
      expect(subscriptions.find('details').exists()).toBe(false)
    } else {
      expect(subscriptions.get('details summary').text()).toBe('Expired / inactive legacy packages (1)')
      expect(subscriptions.get('details [data-legacy-id="7"]').exists()).toBe(true)
    }
  })

  it('keeps independent legacy and new-package loading errors retryable', async () => {
    mocks.mine.mockRejectedValueOnce(new Error('packages offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('#subscriptions').text()).toContain('Legacy card')
    expect(wrapper.get('#subscriptions').text()).not.toContain('No subscription packages yet')
    expect(wrapper.get('#subscriptions [role="alert"]').text()).toContain('packages offline')
    await wrapper.get('#subscriptions [role="alert"] button').trigger('click')
    await flushPromises()
    expect(order(wrapper)).toEqual(['1', '2'])
  })

  it('saves package order, locks duplicate moves and retains order after reload', async () => {
    const saved = deferred<void>()
    mocks.reorder.mockReturnValueOnce(saved.promise)
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button[aria-label="Move Card 1 down"]').trigger('click')
    expect(wrapper.get('button[aria-label="Move Card 2 down"]').attributes('disabled')).toBeDefined()
    await wrapper.get('button[aria-label="Move Card 2 down"]').trigger('click')
    expect(mocks.reorder).toHaveBeenCalledTimes(1)
    expect(mocks.reorder).toHaveBeenCalledWith([2, 1])
    mocks.mine.mockResolvedValue([holding(2), holding(1)])
    saved.resolve()
    await flushPromises()
    await wrapper.get('button[aria-label="Refresh"]').trigger('click')
    await flushPromises()
    expect(order(wrapper)).toEqual(['2', '1'])
    const reopened = render()
    await flushPromises()
    expect(order(reopened)).toEqual(['2', '1'])
  })

  it('queues an explicit holdings refresh across an unchanged drag', async () => {
    const wrapper = render()
    await flushPromises()
    const owned = wrapper.getComponent(OwnedPackages)
    owned.getComponent({ name: 'VueDraggable' }).vm.$emit('start')
    owned.vm.refresh()
    expect(mocks.mine).toHaveBeenCalledTimes(1)
    owned.getComponent({ name: 'VueDraggable' }).vm.$emit('end')
    await flushPromises()
    expect(mocks.mine).toHaveBeenCalledTimes(2)
    expect(mocks.reorder).not.toHaveBeenCalled()
  })

  it('redeems once and refreshes balance, both holdings and history after success', async () => {
    const payment = deferred<{ message: string; type: string; value: number }>()
    mocks.redeem.mockReturnValueOnce(payment.promise)
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#redeem-code').setValue('  CODE  ')
    await wrapper.get('#redeem form').trigger('submit')
    await wrapper.get('#redeem form').trigger('submit')
    expect(mocks.redeem).toHaveBeenCalledTimes(1)
    expect(mocks.redeem).toHaveBeenCalledWith('CODE')
    mocks.refreshUser.mockImplementation(async () => { mocks.auth.user!.balance = 28 })
    payment.resolve({ message: 'Applied', type: 'subscription', value: 7 })
    await flushPromises()
    expect((wrapper.get('#redeem-code').element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('#redeem [role="status"]').text()).toContain('Applied')
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('$28.00')
    expect(mocks.mine).toHaveBeenCalledTimes(2)
    expect(mocks.legacy).toHaveBeenCalledTimes(2)
    expect(mocks.history).toHaveBeenCalledTimes(2)
    expect(mocks.activeSubscriptions).toHaveBeenCalledWith(true)
  })

  it('does not turn a confirmed redemption into a failure when account refresh fails', async () => {
    const wrapper = render()
    await flushPromises()
    mocks.refreshUser.mockRejectedValueOnce(new Error('offline'))
    await wrapper.get('#redeem-code').setValue('CODE')
    await wrapper.get('#redeem form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('#redeem [role="status"]').text()).toContain('Applied')
    expect(wrapper.get('#redeem [role="alert"]').text()).toContain('Code redeemed successfully')
    expect((wrapper.get('#redeem-code').element as HTMLInputElement).value).toBe('')
    expect(mocks.error).not.toHaveBeenCalled()
    await wrapper.get('#redeem [role="alert"] button').trigger('click')
    await flushPromises()
    expect(mocks.redeem).toHaveBeenCalledTimes(1)
    expect(wrapper.find('#redeem [role="alert"]').exists()).toBe(false)
  })

  it('retains a rejected code for retry and keeps history errors distinct from empty history', async () => {
    mocks.history.mockRejectedValueOnce(new Error('offline'))
    mocks.redeem.mockRejectedValueOnce({ response: { data: { detail: 'Invalid code' } } })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('#redeem details [role="alert"]').text()).toContain('Could not load redemption history')
    expect(wrapper.get('#redeem details').text()).not.toContain('will appear')
    await wrapper.get('#redeem-code').setValue('CODE')
    await wrapper.get('#redeem form').trigger('submit')
    await flushPromises()
    expect((wrapper.get('#redeem-code').element as HTMLInputElement).value).toBe('CODE')
    expect(wrapper.get('#redeem').text()).toContain('Invalid code')
    await wrapper.get('#redeem form').trigger('submit')
    await flushPromises()
    expect(mocks.redeem).toHaveBeenCalledTimes(2)
    expect(wrapper.find('#redeem [role="alert"]').exists()).toBe(false)
  })

  it('waits for an account refresh retry before accepting another redemption', async () => {
    const wrapper = render()
    await flushPromises()
    mocks.refreshUser.mockRejectedValueOnce(new Error('offline'))
    await wrapper.get('#redeem-code').setValue('FIRST')
    await wrapper.get('#redeem form').trigger('submit')
    await flushPromises()

    const refresh = deferred<void>()
    mocks.refreshUser.mockReturnValueOnce(refresh.promise)
    await wrapper.get('#redeem [role="alert"] button').trigger('click')
    await wrapper.get('#redeem-code').setValue('SECOND')
    expect(wrapper.get('#redeem button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#redeem form').trigger('submit')
    expect(mocks.redeem).toHaveBeenCalledTimes(1)

    refresh.resolve()
    await flushPromises()
    mocks.redeem.mockResolvedValueOnce({ message: 'Assigned', type: 'subscription', value: 7 })
    await wrapper.get('#redeem form').trigger('submit')
    await flushPromises()
    expect(mocks.redeem).toHaveBeenCalledTimes(2)
    expect(mocks.redeem).toHaveBeenLastCalledWith('SECOND')
    expect(mocks.activeSubscriptions).toHaveBeenCalledWith(true)
  })

  it.each([false, true])('hides disabled recharge without hiding already-purchased holdings (sales disabled=%s)', async disabled => {
    if (disabled) mocks.config.mockResolvedValue({ data: { enabled: true, balance_disabled: true } })
    else mocks.app.cachedPublicSettings.payment_enabled = false
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('a[href="/purchase?tab=recharge"]').exists()).toBe(false)
    expect(order(wrapper)).toEqual(['1', '2'])
    expect(wrapper.find('a[href="/purchase?tab=subscription"]').exists()).toBe(disabled)
    if (!disabled) expect(mocks.config).not.toHaveBeenCalled()
  })

  it('hides recharge when the payment config disables payment despite an enabled public flag', async () => {
    mocks.config.mockResolvedValue({ data: { enabled: false, balance_disabled: false } })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('a[href="/purchase?tab=recharge"]').exists()).toBe(false)
    expect(order(wrapper)).toEqual(['1', '2'])
  })

  it('keeps recharge hidden when payment config cannot be loaded', async () => {
    mocks.config.mockRejectedValueOnce(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('a[href="/purchase?tab=recharge"]').exists()).toBe(false)
    expect(order(wrapper)).toEqual(['1', '2'])
  })

  it('does not replace newer disabled payment config with an older enabled response', async () => {
    const older = deferred<{ data: { enabled: boolean; balance_disabled: boolean } }>()
    mocks.config.mockReturnValueOnce(older.promise)
    const wrapper = render()
    await flushPromises()
    mocks.app.cachedPublicSettings.payment_enabled = false
    await flushPromises()
    mocks.config.mockResolvedValueOnce({ data: { enabled: true, balance_disabled: true } })
    mocks.app.cachedPublicSettings.payment_enabled = true
    await flushPromises()
    expect(mocks.config).toHaveBeenCalledTimes(2)
    older.resolve({ data: { enabled: true, balance_disabled: false } })
    await flushPromises()
    expect(wrapper.find('a[href="/purchase?tab=recharge"]').exists()).toBe(false)
  })

  it('honors simple mode without requesting hidden holdings, redemption, quota or payment data', async () => {
    mocks.auth.isSimpleMode = true
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('#subscriptions').exists()).toBe(false)
    expect(wrapper.find('#redeem').exists()).toBe(false)
    expect(wrapper.find('a[href="/purchase?tab=recharge"]').exists()).toBe(false)
    expect(mocks.mine).not.toHaveBeenCalled()
    expect(mocks.legacy).not.toHaveBeenCalled()
    expect(mocks.history).not.toHaveBeenCalled()
    expect(mocks.quotas).not.toHaveBeenCalled()
    expect(mocks.config).not.toHaveBeenCalled()
  })

  it('retains platform quota limits, disabled windows and reset time without usage charts', async () => {
    mocks.quotas.mockResolvedValue({ platform_quotas: [{ platform: 'openai', daily_limit_usd: 0, daily_usage_usd: 0, weekly_limit_usd: 10, weekly_usage_usd: 4, weekly_window_resets_at: '2026-09-20T00:00:00Z' }] })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="platform-quotas"]').text()).toContain('Disabled')
    expect(wrapper.get('[data-testid="platform-quotas"]').text()).toContain('$4.00 / $10.00')
    expect(wrapper.get('[data-testid="platform-quotas"] progress').attributes('value')).toBe('4')
  })

  it('keeps old subscription and redemption view bodies accessible under one layout', async () => {
    const subscriptions = render(SubscriptionsView)
    const redeem = render(RedeemView)
    await flushPromises()
    expect(subscriptions.findAll('main')).toHaveLength(1)
    expect(subscriptions.get('#subscriptions').text()).toContain('Legacy card')
    expect(redeem.findAll('main')).toHaveLength(1)
    expect(redeem.get('#redeem form').exists()).toBe(true)
    expect(redeem.get('#redeem details').exists()).toBe(true)
  })
})
