import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import PackagePaymentDialog from '../PackagePaymentDialog.vue'
import PackageGroupsView from '@/views/user/PackageGroupsView.vue'
import PaymentView from '@/views/user/PaymentView.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import { createPaymentRecoverySnapshot, PAYMENT_RECOVERY_STORAGE_KEY } from '@/components/payment/paymentFlow'
import type { PackageGroupBuy } from '@/types/packages'
import type { CreateOrderResult } from '@/types/payment'

const mocks = vi.hoisted(() => ({ checkout: vi.fn(), group: vi.fn(), plans: vi.fn(), start: vi.fn(), create: vi.fn(), poll: vi.fn(), cancel: vi.fn(), push: vi.fn(), replace: vi.fn(), error: vi.fn(), mobile: vi.fn(), query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: '4' }, query: mocks.query, path: '/package-groups/4' }), useRouter: () => ({ push: mocks.push, replace: mocks.replace, resolve: () => ({ href: '/payment/provider' }) }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/payment', () => ({ paymentAPI: { getCheckoutInfo: mocks.checkout, cancelOrder: mocks.cancel } }))
vi.mock('@/api/packages', () => ({ packagesAPI: { group: mocks.group, plans: mocks.plans, startGroup: mocks.start } }))
vi.mock('@/stores/payment', () => ({ usePaymentStore: () => ({ createOrder: mocks.create, pollOrderStatus: mocks.poll }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: {}, refreshUser: vi.fn() }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ fetchActiveSubscriptions: vi.fn().mockResolvedValue([]) }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError: mocks.error, showInfo: vi.fn(), showWarning: vi.fn() }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.error }) }))
vi.mock('@/utils/device', () => ({ isMobileDevice: mocks.mobile }))
vi.mock('qrcode', () => ({ default: { toCanvas: vi.fn() } }))

const group: PackageGroupBuy = {
  id: 4, status: 'draft', paid_count: 0, target_members: 3, joined: false, order_id: null,
  starts_at: null, ends_at: null, settled_at: null, final_members: null, final_quota_usd: null,
  plan: { id: 2, name: 'Monthly package', group_id: 5, group_name: 'Models', group_platform: 'openai', description: '', validity_days: 30, base_quota_usd: 1800, price: 385, currency: 'CNY', group_buy_enabled: true, group_buy_hours: 48, tiers: [{ members: 3, quota_usd: 1980 }], for_sale: true, sort_order: 0 },
}
const order: CreateOrderResult = { order_id: 10, amount: 385, pay_amount: 385, fee_rate: 0, qr_code: 'test-qr', expires_at: '2099-01-01T00:00:00Z', currency: 'CNY' }
const method = { available: true, daily_limit: 0, daily_remaining: 0, daily_used: 0, single_min: 0, single_max: 0, fee_rate: 0, currency: 'CNY' }
const wrappers: ReturnType<typeof mount>[] = []
async function mountDialog(termsAccepted = true) {
  const wrapper = mount(PackagePaymentDialog, { attachTo: document.body, props: { show: true, group, termsAccepted }, global: { stubs: { Transition: false, RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}
function page() { return new DOMWrapper(document.body) }
function submit(_wrapper: ReturnType<typeof mount>) {
  return page().findAll('button').find(button => button.text().includes('payment.createOrder'))!
}
async function mountShop() {
  mocks.query = { tab: 'subscription' }
  const wrapper = mount(PaymentView, { attachTo: document.body, global: { stubs: { Transition: false, AppLayout: { template: '<div><slot /></div>' }, RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}
async function openSingle(wrapper: ReturnType<typeof mount>, index = 0) {
  await wrapper.findAll('button').filter(button => button.text() === 'packages.single')[index].trigger('click')
  await flushPromises()
}

describe('in-place group payment', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-11T00:00:00Z'))
    localStorage.clear()
    mocks.query = { group_buy_id: '999', wechat_resume_token: 'unrelated' }
    mocks.group.mockReset().mockResolvedValue(group)
    mocks.checkout.mockReset().mockResolvedValue({ data: { methods: { alipay: method }, plans: [], balance_disabled: false, recharge_fee_rate: 0 } })
    mocks.create.mockReset().mockResolvedValue(order)
    mocks.poll.mockReset().mockResolvedValue({ id: 10, status: 'PENDING', amount: 385, pay_amount: 385 })
    mocks.cancel.mockReset().mockResolvedValue({})
    mocks.plans.mockReset().mockResolvedValue([group.plan])
    mocks.start.mockReset().mockResolvedValue(group)
    mocks.mobile.mockReturnValue(false)
  })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.unstubAllGlobals(); vi.useRealTimers() })

  it('shows compact checkout, carries consent and submits the group without route navigation', async () => {
    const wrapper = await mountDialog()
    expect(page().text()).toContain('Monthly package')
    expect(page().find('input[type="checkbox"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'PackagePlanCard' }).exists()).toBe(false)
    expect(submit(wrapper).attributes('disabled')).toBeUndefined()
    await submit(wrapper).trigger('click')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledTimes(1)
    expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({ amount: 385, order_type: 'package', package_plan_id: 2, group_buy_id: 4 }))
    expect(mocks.create.mock.calls[0][0]).not.toHaveProperty('plan_id')
    expect(wrapper.getComponent(PaymentStatusPanel).props('orderId')).toBe(10)
    expect(mocks.group).toHaveBeenCalledWith(4)
    expect(mocks.push).not.toHaveBeenCalled()
    expect(mocks.replace).not.toHaveBeenCalled()
  })

  it('requires consent only when it was not accepted on the source page', async () => {
    const wrapper = await mountDialog(false)
    expect(submit(wrapper).attributes('disabled')).toBeDefined()
    await page().get('input[type="checkbox"]').setValue(true)
    expect(submit(wrapper).attributes('disabled')).toBeUndefined()
    await page().findAll('button').find(button => button.text() === 'common.cancel')!.trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(mocks.create).not.toHaveBeenCalled()
    expect(mocks.push).not.toHaveBeenCalled()
  })

  it('locks submit and close while creation is pending and freezes the source group', async () => {
    let resolve!: (value: CreateOrderResult) => void
    mocks.create.mockReturnValueOnce(new Promise<CreateOrderResult>(res => { resolve = res }))
    const wrapper = await mountDialog()
    const button = submit(wrapper)
    await button.trigger('click')
    await button.trigger('click')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await page().get('[role="dialog"]').trigger('click')
    expect(page().find('button[aria-label="Close modal"]').exists()).toBe(false)
    expect(wrapper.emitted('close')).toBeUndefined()
    await wrapper.setProps({ group: { ...group, id: 77, plan: { ...group.plan, id: 88 } } })
    resolve(order)
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledTimes(1)
    expect(mocks.create.mock.calls[0][0]).toMatchObject({ group_buy_id: 4, package_plan_id: 2 })
    expect(wrapper.getComponent(PaymentStatusPanel).props('orderId')).toBe(10)
  })

  it('reopens only its existing pending order without cancelling or creating another one', async () => {
    const wrapper = await mountDialog()
    await submit(wrapper).trigger('click')
    await flushPromises()
    await page().get('button[aria-label="Close modal"]').trigger('click')
    expect(mocks.cancel).not.toHaveBeenCalled()
    await wrapper.setProps({ show: false })
    mocks.group.mockResolvedValue({ ...group, order_id: 10 })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(wrapper.getComponent(PaymentStatusPanel).props('orderId')).toBe(10)
    expect(submit(wrapper)).toBeUndefined()
    expect(mocks.create).toHaveBeenCalledTimes(1)
    expect(mocks.push).not.toHaveBeenCalled()
  })

  it('does not adopt or remove an unrelated recovery order', async () => {
    const seed = await mountDialog()
    await submit(seed).trigger('click')
    await flushPromises()
    seed.unmount()
    wrappers.splice(wrappers.indexOf(seed), 1)
    mocks.group.mockResolvedValue({ ...group, order_id: 19 })
    const wrapper = await mountDialog()
    expect(page().text()).toContain('packages.existingPayment')
    expect(wrapper.findComponent(PaymentStatusPanel).exists()).toBe(false)
    expect(submit(wrapper)).toBeUndefined()
    await page().get('button[aria-label="Close modal"]').trigger('click')
    expect(JSON.parse(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)!).orderId).toBe(10)
    expect(mocks.create).toHaveBeenCalledTimes(1)
  })

  it.each(['COMPLETED', 'CANCELLED'])('keeps %s outcome in the modal until confirmation', async status => {
    const wrapper = await mountDialog()
    await submit(wrapper).trigger('click')
    await flushPromises()
    if (status === 'CANCELLED') {
      await page().findAll('button').find(button => button.text() === 'payment.qr.cancelOrder')!.trigger('click')
    } else {
      mocks.poll.mockResolvedValue({ id: 10, status, amount: 385, pay_amount: 385 })
      await vi.advanceTimersByTimeAsync(3000)
    }
    await flushPromises()
    expect(page().text()).toContain(status === 'COMPLETED' ? 'payment.result.success' : 'payment.qr.cancelled')
    expect(wrapper.emitted('success')?.length ?? 0).toBe(status === 'COMPLETED' ? 1 : 0)
    expect(mocks.push).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toBeUndefined()
    await page().findAll('button').find(button => button.text() === 'common.confirm')!.trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('shows unavailable methods and retries a failed checkout load without submitting', async () => {
    mocks.checkout.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await mountDialog()
    expect(page().get('[role="alert"]').text()).toContain('offline')
    mocks.checkout.mockResolvedValue({ data: { methods: {}, plans: [], recharge_fee_rate: 0 } })
    await page().get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(page().text()).toContain('payment.notAvailable')
    expect(submit(wrapper).attributes('disabled')).toBeDefined()
    expect(mocks.create).not.toHaveBeenCalled()
  })

  it.each(['draft', 'open'] as const)('opens payment from the %s detail and keeps the detail beneath it', async status => {
    mocks.group.mockResolvedValue({ ...group, status, ends_at: status === 'open' ? '2099-01-01T00:00:00Z' : null })
    const wrapper = mount(PackageGroupsView, { attachTo: document.body, global: { stubs: { Transition: false, AppLayout: { template: '<div><slot /></div>' }, RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
    wrappers.push(wrapper)
    await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('.package-detail__action button').trigger('click')
    await flushPromises()
    expect(page().find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.find('.package-detail__product').exists()).toBe(true)
    expect(page().findAll('input[type="checkbox"]')).toHaveLength(1)
    await submit(wrapper).trigger('click')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({ package_plan_id: 2, group_buy_id: 4 }))
    expect(mocks.push).not.toHaveBeenCalled()
    expect(mocks.replace).not.toHaveBeenCalled()
  })

  it('refreshes the real source detail after verified payment without dismissing its result', async () => {
    const wrapper = mount(PackageGroupsView, { attachTo: document.body, global: { stubs: { Transition: false, AppLayout: { template: '<div><slot /></div>' }, RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
    wrappers.push(wrapper)
    await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('.package-detail__action button').trigger('click')
    await flushPromises()
    await submit(wrapper).trigger('click')
    await flushPromises()
    mocks.group.mockResolvedValue({ ...group, status: 'open', joined: true, paid_count: 1, order_id: 10, ends_at: '2099-01-01T00:00:00Z' })
    mocks.poll.mockResolvedValue({ id: 10, status: 'COMPLETED', amount: 385, pay_amount: 385 })
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(mocks.group).toHaveBeenCalledTimes(3)
    expect(wrapper.find('.package-detail__action a[href="/orders"]').exists()).toBe(true)
    expect(page().get('[role="dialog"]').text()).toContain('payment.result.success')
    expect(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
    expect(mocks.push).not.toHaveBeenCalled()
    expect(mocks.replace).not.toHaveBeenCalled()
  })

  it.each(['ok', 'cancel', 'fail'])('keeps one admitted order when mobile JSAPI reports %s', async outcome => {
    mocks.mobile.mockReturnValue(true)
    mocks.checkout.mockResolvedValue({ data: { methods: { wxpay: method }, plans: [], balance_disabled: false, recharge_fee_rate: 0 } })
    const jsapi = { appId: 'test-app', timeStamp: '1', nonceStr: 'test-nonce', package: 'prepay_id=test', signType: 'RSA', paySign: 'test-signature' }
    mocks.create.mockResolvedValue({ ...order, qr_code: '', result_type: 'jsapi_ready', jsapi })
    const invoke = vi.fn((_action, _payload, callback) => callback({ err_msg: `get_brand_wcpay_request:${outcome}` }))
    vi.stubGlobal('WeixinJSBridge', { invoke })
    const wrapper = await mountDialog()
    await submit(wrapper).trigger('click')
    await flushPromises()
    expect(invoke).toHaveBeenCalledWith('getBrandWCPayRequest', jsapi, expect.any(Function))
    expect(wrapper.getComponent(PaymentStatusPanel).props('orderId')).toBe(10)
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(submit(wrapper)).toBeUndefined()
    expect(JSON.parse(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)!).orderId).toBe(10)
    expect(mocks.create).toHaveBeenCalledTimes(1)
    expect(mocks.cancel).not.toHaveBeenCalled()
    expect(mocks.push).not.toHaveBeenCalled()
    expect(mocks.replace).not.toHaveBeenCalled()
  })

  it('opens group details from the shop without opening or creating a payment', async () => {
    mocks.query = { tab: 'subscription' }
    const wrapper = mount(PaymentView, { attachTo: document.body, global: { stubs: { Transition: false, AppLayout: { template: '<div><slot /></div>' }, RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
    wrappers.push(wrapper)
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'packages.start')!.trigger('click')
    await flushPromises()
    expect(page().find('[role="dialog"]').exists()).toBe(false)
    expect(mocks.start).toHaveBeenCalledWith(2)
    expect(mocks.create).not.toHaveBeenCalled()
    expect(mocks.push).toHaveBeenCalledWith('/package-groups/4')
    expect(mocks.replace).not.toHaveBeenCalled()

    mocks.start.mockResolvedValue({ ...group, id: 8 })
    mocks.group.mockResolvedValue({ ...group, status: 'open', joined: true, order_id: 10 })
    await wrapper.findAll('button').find(button => button.text() === 'packages.start')!.trigger('click')
    await flushPromises()
    expect(mocks.start).toHaveBeenCalledTimes(2)
    expect(mocks.push).toHaveBeenLastCalledWith('/package-groups/8')
    expect(mocks.group).not.toHaveBeenCalled()
    expect(mocks.create).not.toHaveBeenCalled()
    expect(page().find('[role="dialog"]').exists()).toBe(false)
  })

  it('opens individual checkout over the shop, requires standalone consent and submits without a group', async () => {
    const wrapper = await mountShop()
    await openSingle(wrapper)
    const dialog = page().get('[role="dialog"]')
    expect(dialog.text()).toContain('Monthly package')
    expect(dialog.text()).toContain('packages.singleTerms')
    expect(dialog.text()).toContain('packages.monthSchedule')
    expect(dialog.text()).not.toContain('packages.settlement')
    expect(dialog.text()).not.toContain('packages.members')
    expect(wrapper.findComponent({ name: 'PackageShop' }).exists()).toBe(true)
    expect(submit(wrapper).attributes('disabled')).toBeDefined()
    expect(mocks.create).not.toHaveBeenCalled()
    await dialog.get('input[type="checkbox"]').setValue(true)
    await submit(wrapper).trigger('click')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledTimes(1)
    expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({ amount: 385, order_type: 'package', package_plan_id: 2 }))
    expect(mocks.create.mock.calls[0][0]).not.toHaveProperty('group_buy_id')
    expect(mocks.create.mock.calls[0][0]).not.toHaveProperty('plan_id')
    expect(wrapper.getComponent(PaymentStatusPanel).props('orderId')).toBe(10)
    expect(mocks.start).not.toHaveBeenCalled()
    expect(mocks.group).not.toHaveBeenCalled()
    expect(mocks.push).not.toHaveBeenCalled()
    expect(mocks.replace).not.toHaveBeenCalled()
  })

  it('closes before payment in the shop and locks duplicate submits, escape and backdrop during payment creation', async () => {
    const wrapper = await mountShop()
    await openSingle(wrapper)
    await page().get('button[aria-label="Close modal"]').trigger('click')
    await flushPromises()
    expect(page().find('[role="dialog"]').exists()).toBe(false)
    expect(mocks.create).not.toHaveBeenCalled()
    await openSingle(wrapper)
    await page().get('input[type="checkbox"]').setValue(true)
    let resolve!: (value: CreateOrderResult) => void
    mocks.create.mockReturnValueOnce(new Promise<CreateOrderResult>(res => { resolve = res }))
    const button = submit(wrapper)
    await button.trigger('click')
    await button.trigger('click')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await page().get('[role="dialog"]').trigger('click')
    expect(page().find('button[aria-label="Close modal"]').exists()).toBe(false)
    expect(page().find('[role="dialog"]').exists()).toBe(true)
    resolve(order)
    await flushPromises()
    await page().get('button[aria-label="Close modal"]').trigger('click')
    await flushPromises()
    expect(page().find('[role="dialog"]').exists()).toBe(false)
    await openSingle(wrapper)
    expect(wrapper.getComponent(PaymentStatusPanel).props('orderId')).toBe(10)
    expect(submit(wrapper)).toBeUndefined()
    expect(mocks.create).toHaveBeenCalledTimes(1)
    expect(mocks.cancel).not.toHaveBeenCalled()
    expect(mocks.push).not.toHaveBeenCalled()
  })

  it('keeps each individual plan pending order when switching plans, without adopting another plan recovery', async () => {
    mocks.plans.mockResolvedValue([group.plan, { ...group.plan, id: 3, name: 'Other package', price: 200 }])
    const wrapper = await mountShop()
    await openSingle(wrapper)
    await page().get('input[type="checkbox"]').setValue(true)
    await submit(wrapper).trigger('click')
    await flushPromises()
    await page().get('button[aria-label="Close modal"]').trigger('click')
    await flushPromises()
    await openSingle(wrapper, 1)
    expect(page().get('[role="dialog"]').text()).toContain('Other package')
    expect(wrapper.findComponent(PaymentStatusPanel).exists()).toBe(false)
    mocks.create.mockResolvedValueOnce({ ...order, order_id: 11, amount: 200, pay_amount: 200 })
    await page().get('input[type="checkbox"]').setValue(true)
    await submit(wrapper).trigger('click')
    await flushPromises()
    expect(mocks.create.mock.calls[1][0]).toMatchObject({ package_plan_id: 3, amount: 200 })
    await page().get('button[aria-label="Close modal"]').trigger('click')
    await flushPromises()
    await openSingle(wrapper)
    expect(wrapper.getComponent(PaymentStatusPanel).props()).toMatchObject({ orderId: 10, amount: 385 })
    expect(mocks.create).toHaveBeenCalledTimes(2)
    expect(JSON.parse(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)!).orderId).toBe(11)
  })

  it.each(['balance', 'subscription', 'package'] as const)('ignores unrelated %s recovery and route queries in a new individual modal', async orderType => {
    const wrapper = await mountShop()
    mocks.query = { tab: 'recharge', group_buy_id: '999', package_plan_id: '777', wechat_resume_token: 'other' }
    const unrelated = createPaymentRecoverySnapshot({ orderId: 99, amount: 17, qrCode: 'other', expiresAt: order.expires_at, paymentType: 'alipay', payUrl: '', outTradeNo: '', clientSecret: '', intentId: '', currency: 'CNY', countryCode: '', paymentEnv: '', payAmount: 17, orderType, paymentMode: '', resumeToken: '' })
    localStorage.setItem(PAYMENT_RECOVERY_STORAGE_KEY, JSON.stringify(unrelated))
    await openSingle(wrapper)
    expect(page().get('[role="dialog"]').text()).toContain('Monthly package')
    expect(wrapper.findComponent(PaymentStatusPanel).exists()).toBe(false)
    expect(mocks.group).not.toHaveBeenCalled()
    expect(mocks.create).not.toHaveBeenCalled()
    expect(mocks.push).not.toHaveBeenCalled()
    expect(mocks.replace).not.toHaveBeenCalled()
    await page().get('button[aria-label="Close modal"]').trigger('click')
    expect(JSON.parse(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)!).orderId).toBe(99)
  })

  it('keeps success over the shop, then permits another independent purchase of the same plan', async () => {
    const wrapper = await mountShop()
    await openSingle(wrapper)
    await page().get('input[type="checkbox"]').setValue(true)
    await submit(wrapper).trigger('click')
    await flushPromises()
    mocks.poll.mockResolvedValue({ id: 10, status: 'COMPLETED', amount: 385, pay_amount: 385 })
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(page().get('[role="dialog"]').text()).toContain('payment.result.success')
    expect(wrapper.findComponent({ name: 'PackageShop' }).exists()).toBe(true)
    expect(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
    expect(mocks.push).not.toHaveBeenCalled()
    await page().findAll('button').find(button => button.text() === 'common.confirm')!.trigger('click')
    await flushPromises()
    await openSingle(wrapper)
    expect(wrapper.findComponent(PaymentStatusPanel).exists()).toBe(false)
    expect(submit(wrapper).attributes('disabled')).toBeDefined()
    await page().get('input[type="checkbox"]').setValue(true)
    await submit(wrapper).trigger('click')
    await flushPromises()
    expect(mocks.create).toHaveBeenCalledTimes(2)
    expect(mocks.start).not.toHaveBeenCalled()
  })
})
