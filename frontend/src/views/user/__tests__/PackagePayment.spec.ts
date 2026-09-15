import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import PaymentView from '../PaymentView.vue'
import PackagePlanCard from '@/components/packages/PackagePlanCard.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import type { PackagePlan } from '@/types/packages'
const mocks = vi.hoisted(() => ({ query: {} as Record<string, string>, plans: vi.fn(), group: vi.fn(), checkout: vi.fn(), createOrder: vi.fn(), push: vi.fn(), replace: vi.fn(), error: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ path: '/purchase', query: mocks.query }), useRouter: () => ({ push: mocks.push, replace: mocks.replace, resolve: () => ({ href: '/payment/stripe' }) }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/packages', () => ({ packagesAPI: { plans: mocks.plans, group: mocks.group } }))
vi.mock('@/api/payment', () => ({ paymentAPI: { getCheckoutInfo: mocks.checkout } }))
vi.mock('@/stores/payment', () => ({ usePaymentStore: () => ({ createOrder: mocks.createOrder }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ fetchActiveSubscriptions: vi.fn().mockResolvedValue([]) }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: {}, refreshUser: vi.fn() }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError: mocks.error }) }))
vi.mock('@/utils/device', () => ({ isMobileDevice: () => false }))
const plan: PackagePlan = { id: 2, name: 'Monthly', group_id: 5, group_name: 'Models', group_platform: 'openai', description: '', validity_days: 30, base_quota_usd: 1800, price: 385, currency: 'CNY', group_buy_enabled: true, group_buy_hours: 48, tiers: [{ members: 3, quota_usd: 1980 }], for_sale: true, sort_order: 0 }
const method = { available: true, daily_limit: 0, daily_remaining: 0, daily_used: 0, single_min: 0, single_max: 0, fee_rate: 0 }
async function mountView() {
  const wrapper = shallowMount(PaymentView, { global: { stubs: { RouterLink: true, PaymentCheckout: false, AppLayout: { template: '<div><slot /></div>' }, Teleport: true } } })
  await flushPromises()
  return wrapper
}
describe('package checkout uses existing payment flow', () => {
  beforeEach(() => {
    localStorage.clear()
    mocks.query = { tab: 'subscription', package_plan_id: '2', group_buy_id: '4' }
    mocks.plans.mockResolvedValue([plan])
    mocks.group.mockResolvedValue({ id: 4, plan, status: 'open', paid_count: 1, target_members: 3, joined: false, order_id: null, ends_at: '2099-01-01T00:00:00Z' })
    mocks.checkout.mockResolvedValue({ data: { methods: { alipay: { ...method, currency: 'CNY' }, stripe: { ...method, currency: 'USD' } }, plans: [], balance_disabled: false, recharge_fee_rate: 0, subscription_usd_to_cny_rate: 7, balance_recharge_multiplier: 0.2 } })
    mocks.createOrder.mockReset().mockResolvedValue({ order_id: 10, amount: 385, pay_amount: 385, fee_rate: 0, qr_code: 'test-qr', expires_at: '2099-01-01T00:00:00Z' })
    mocks.error.mockReset()
    mocks.replace.mockReset().mockResolvedValue(undefined)
  })
  it('requires consent, excludes incompatible currencies, and submits the snapshotted group package without legacy FX', async () => {
    const wrapper = await mountView()
    expect(wrapper.getComponent(PackagePlanCard).props('plan')).toEqual(plan)
    expect(wrapper.getComponent(PaymentMethodSelector).props('methods')).toEqual(expect.arrayContaining([expect.objectContaining({ type: 'stripe', available: false }), expect.objectContaining({ type: 'alipay', available: true })]))
    const submit = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))!
    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await submit.trigger('click')
    await flushPromises()
    expect(mocks.createOrder).toHaveBeenCalledWith(expect.objectContaining({ amount: 385, order_type: 'package', package_plan_id: 2, group_buy_id: 4 }))
    expect(mocks.createOrder.mock.calls[0][0]).not.toHaveProperty('plan_id')
    expect(mocks.error).not.toHaveBeenCalled()
  })
  it('does not submit when every method uses another currency', async () => {
    mocks.group.mockResolvedValue({ id: 4, plan: { ...plan, currency: 'NZD' }, status: 'open', joined: false, ends_at: '2099-01-01T00:00:00Z' })
    const wrapper = await mountView()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.text()).toContain('packages.currencyMismatch')
    expect(wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))!.attributes('disabled')).toBeDefined()
    expect(mocks.createOrder).not.toHaveBeenCalled()
  })
  it('sends an existing group order to the ordinary order page instead of preparing another payment', async () => {
    mocks.group.mockResolvedValue({ id: 4, plan, status: 'open', joined: false, order_id: 17, ends_at: '2099-01-01T00:00:00Z' })
    const wrapper = await mountView()
    expect(mocks.replace).toHaveBeenCalledWith('/orders')
    expect(mocks.createOrder).not.toHaveBeenCalled()
    expect(wrapper.findComponent(PackagePlanCard).exists()).toBe(false)
    wrapper.unmount()
  })
  it('rejects expiry between clock ticks, then displays the closed group state', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2099-01-01T00:00:00Z'))
    mocks.group.mockResolvedValue({ id: 4, plan, status: 'open', joined: false, ends_at: '2099-01-01T00:00:01Z' })
    const wrapper = await mountView()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    const submit = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))!
    expect(submit.attributes('disabled')).toBeUndefined()
    vi.setSystemTime(new Date('2099-01-01T00:00:02Z'))
    await submit.trigger('click')
    expect(mocks.createOrder).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1000)
    await wrapper.vm.$nextTick()
    expect(submit.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[role="status"]').text()).toContain('packages.closing')
    wrapper.unmount()
    vi.useRealTimers()
  })
  it('locks cancellation while creating a package order and ignores duplicate clicks', async () => {
    let resolveOrder!: (value: unknown) => void
    mocks.createOrder.mockImplementationOnce(() => new Promise(resolve => { resolveOrder = resolve }))
    const wrapper = await mountView()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    const submit = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))!
    await submit.trigger('click')
    await wrapper.vm.$nextTick()
    const cancel = wrapper.findAll('button').find(button => button.text().includes('common.cancel'))!
    expect(cancel.attributes('disabled')).toBeDefined()
    await cancel.trigger('click')
    expect(mocks.replace).not.toHaveBeenCalled()
    const methods = wrapper.getComponent(PaymentMethodSelector)
    const selectedMethod = methods.props('selected')
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    methods.vm.$emit('select', 'stripe')
    await wrapper.vm.$nextTick()
    expect(methods.props('selected')).toBe(selectedMethod)
    await submit.trigger('click')
    expect(mocks.createOrder).toHaveBeenCalledTimes(1)
    resolveOrder({ order_id: 10, amount: 385, pay_amount: 385, fee_rate: 0, qr_code: 'test-qr', expires_at: '2099-01-01T00:00:00Z' })
    await flushPromises()
    wrapper.unmount()
  })
  it('returns to the group detail when cancelling a group checkout', async () => {
    const wrapper = await mountView()
    await wrapper.findAll('button').find(button => button.text().includes('common.cancel'))!.trigger('click')
    expect(mocks.replace).toHaveBeenCalledWith('/package-groups/4')
    wrapper.unmount()
  })
})
