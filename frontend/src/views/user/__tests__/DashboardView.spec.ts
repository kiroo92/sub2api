import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub, type VueWrapper } from '@vue/test-utils'
import DashboardView from '../DashboardView.vue'
import type { UserDashboardStats } from '@/api/usage'

const mocks = vi.hoisted(() => ({
  stats: vi.fn(), user: vi.fn(), quotas: vi.fn(), config: vi.fn(), list: vi.fn(), unfreeze: vi.fn(),
  invalidate: vi.fn(), showError: vi.fn(),
  auth: { isSimpleMode: false },
  settings: { subscription_enabled: true, subscription_freeze_enabled: false, payment_enabled: true },
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ ...mocks.auth, refreshUser: mocks.user }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: mocks.settings, fetchPublicSettings: async () => mocks.settings, showError: mocks.showError }) }))
vi.mock('@/stores/payment', () => ({ usePaymentStore: () => ({ fetchConfig: mocks.config }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ invalidateCache: mocks.invalidate }) }))
vi.mock('@/api/usage', () => ({ usageAPI: { getDashboardStats: mocks.stats } }))
vi.mock('@/api/user', () => ({ getMyPlatformQuotas: mocks.quotas }))
vi.mock('@/api/subscriptions', () => ({ default: { getMySubscriptions: mocks.list, unfreezeSubscription: mocks.unfreeze } }))
vi.mock('vue-router', async (original) => ({ ...await original<typeof import('vue-router')>(), useRoute: () => ({ hash: '' }) }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const stats: UserDashboardStats = {
  total_api_keys: 1, active_api_keys: 1, total_requests: 4,
  total_input_tokens: 100, total_output_tokens: 200, total_cache_creation_tokens: 300, total_cache_read_tokens: 400,
  total_tokens: 1000, total_cost: 90, total_actual_cost: 3,
  today_requests: 2, today_input_tokens: 100, today_output_tokens: 200, today_cache_creation_tokens: 300, today_cache_read_tokens: 400,
  today_tokens: 1000, today_cost: 90, today_actual_cost: 3, today_subscription_cost: 2.5, today_balance_cost: 0.5,
  average_duration_ms: 0, rpm: 0, tpm: 0,
}
const frozen = {
  id: 10, user_id: 1, group_id: 1, sort_order: 1, status: 'active',
  created_at: '2026-09-01T00:00:00Z', starts_at: '2026-09-01T00:00:00Z', expires_at: '2026-09-25T00:00:00Z',
  frozen_at: '2026-09-20T00:00:00Z', daily_resets_at: '2026-09-21T00:00:00Z',
  daily_usage_usd: 3, weekly_usage_usd: 0, monthly_usage_usd: 0,
  group: { name: 'My frozen plan', platform: 'openai', daily_limit_usd: 10 },
}
let wrapper: VueWrapper | undefined
async function open() {
  wrapper = mount(DashboardView, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' }, RouterLink: RouterLinkStub, Icon: true,
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
  } } })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.resetAllMocks()
  mocks.auth.isSimpleMode = false
  Object.assign(mocks.settings, { subscription_enabled: true, subscription_freeze_enabled: false, payment_enabled: true })
  mocks.stats.mockResolvedValue(stats)
  mocks.user.mockResolvedValue({ balance: 12.34, frozen_balance: 5 })
  mocks.quotas.mockResolvedValue({ platform_quotas: [] })
  mocks.config.mockResolvedValue({ enabled: true, balance_disabled: false })
  mocks.list.mockResolvedValue([frozen])
})
afterEach(() => { wrapper?.unmount() })

describe('user dashboard integration', () => {
  it('renders four real summaries and subscription remaining quota in the same layout', async () => {
    const w = await open()
    expect(w.findAll('.summary-card')).toHaveLength(4)
    expect(w.get('[data-testid="today-cost"]').text()).toContain('$3.0000')
    expect(w.get('[data-testid="today-cost"]').text()).toContain('$2.5000')
    expect(w.get('[data-testid="today-cost"]').text()).not.toContain('90')
    expect(w.get('[data-testid="wallet-balance"]').text()).toContain('$12.34')
    expect(w.get('[data-testid="today-tokens"]').text()).toContain('1.0K')
    expect(w.get('[data-testid="today-tokens"]').text()).toContain('dashboard.cache 700')
    expect(w.get('#subscriptions').text()).toContain('$7.00 / $10.00')
    expect(w.findAllComponents(RouterLinkStub).some(link => link.props('to') === '/purchase?tab=recharge')).toBe(true)
    expect(w.findAll('main')).toHaveLength(1)
  })

  it('keeps thaw available when statistics fail and global freezing is off', async () => {
    mocks.stats.mockRejectedValue(new Error('stats unavailable'))
    mocks.unfreeze.mockResolvedValue({ ...frozen, frozen_at: null, expires_at: '2099-01-01T00:00:00Z' })
    const w = await open()
    expect(w.text()).toContain('dashboard.statsLoadFailed')
    expect(w.get('[data-testid="today-tokens"]').text()).toContain('—')
    await w.findAll('button').find(b => b.text() === 'userSubscriptions.unfreeze')!.trigger('click')
    await w.findAll('button').find(b => b.text() === 'common.confirm')!.trigger('click')
    await flushPromises()
    expect(mocks.unfreeze).toHaveBeenCalledWith(10)
    expect(mocks.invalidate).toHaveBeenCalled()
  })

  it('retries balance independently and does not subtract held funds again', async () => {
    mocks.user.mockRejectedValueOnce(new Error('balance unavailable'))
    const w = await open()
    const wallet = w.get('[data-testid="wallet-balance"]')
    expect(wallet.text()).toContain('dashboard.balanceLoadFailed')
    expect(wallet.text()).not.toContain('$0.00')
    expect(w.get('[data-testid="today-cost"]').text()).toContain('$3.0000')
    await wallet.get('button').trigger('click')
    await flushPromises()
    expect(wallet.text()).toContain('$12.34')
    expect(mocks.stats).toHaveBeenCalledTimes(1)
    expect(mocks.list).toHaveBeenCalledTimes(1)
  })

  it('keeps configured platform quotas accessible even if stats fail', async () => {
    mocks.stats.mockRejectedValue(new Error('stats unavailable'))
    mocks.quotas.mockResolvedValue({ platform_quotas: [{ platform: 'openai', daily_limit_usd: 20, daily_usage_usd: 4 }] })
    const w = await open()
    expect(w.get('details').text()).toContain('$4.00 / $20.00')
    expect(w.get('details').text()).not.toContain('dashboard.todayCost')
    expect(w.get('#subscriptions').text()).toContain('My frozen plan')
  })

  it('shows a retryable subscription error rather than an empty subscription message', async () => {
    mocks.list.mockRejectedValueOnce(new Error('subscriptions unavailable'))
    const w = await open()
    expect(w.get('#subscriptions').text()).toContain('userSubscriptions.failedToLoad')
    expect(w.get('#subscriptions').text()).not.toContain('userSubscriptions.noActiveSubscriptions')
    await w.get('#subscriptions header button').trigger('click')
    await flushPromises()
    expect(w.get('#subscriptions').text()).toContain('My frozen plan')
  })

  it('does not load subscriptions or payment in simple mode', async () => {
    mocks.auth.isSimpleMode = true
    const w = await open()
    expect(w.find('#subscriptions').exists()).toBe(false)
    expect(mocks.list).not.toHaveBeenCalled()
    expect(mocks.config).not.toHaveBeenCalled()
    expect(mocks.quotas).not.toHaveBeenCalled()
  })

  it('does not load subscriptions when the subscription feature is disabled', async () => {
    mocks.settings.subscription_enabled = false
    const w = await open()
    expect(w.find('#subscriptions').exists()).toBe(false)
    expect(mocks.list).not.toHaveBeenCalled()
    expect(w.findAll('.summary-card')).toHaveLength(4)
  })

  it.each([
    ['payment disabled', { enabled: false, balance_disabled: false }],
    ['recharge disabled', { enabled: true, balance_disabled: true }],
    ['config unavailable', null],
  ])('hides recharge when %s', async (_label, config) => {
    mocks.config.mockResolvedValue(config)
    const w = await open()
    expect(w.findAllComponents(RouterLinkStub).some(link => link.props('to') === '/purchase?tab=recharge')).toBe(false)
  })

  it('hides recharge and skips config when the public payment switch is off', async () => {
    mocks.settings.payment_enabled = false
    const w = await open()
    expect(mocks.config).not.toHaveBeenCalled()
    expect(w.findAllComponents(RouterLinkStub).some(link => link.props('to') === '/purchase?tab=recharge')).toBe(false)
  })
})
