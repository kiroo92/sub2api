import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub, type VueWrapper } from '@vue/test-utils'
import PaymentView from '../PaymentView.vue'
import { parseWechatResumeRoute, stripWechatResumeQuery } from '../paymentWechatResume'
import { PAYMENT_RECOVERY_STORAGE_KEY } from '@/components/payment/paymentFlow'
import type { InvoiceQuote } from '@/types/invoice'

const mocks = vi.hoisted(() => ({ quote: vi.fn(), get: vi.fn(), createInvoice: vi.fn(), createOrder: vi.fn(), refreshUser: vi.fn(), replace: vi.fn(), showError: vi.fn(),
  route: { path: '/orders/invoice', query: { selection: 'all' } as Record<string, string> },
}))
vi.mock('vue-router', async (original) => ({ ...await original<typeof import('vue-router')>(), useRoute: () => mocks.route, useRouter: () => ({ replace: mocks.replace, push: vi.fn(), resolve: () => ({ href: '/payment/mock' }) }) }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: 'en' }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { balance: 0 }, refreshUser: mocks.refreshUser }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ cachedPublicSettings: { subscription_enabled: false }, showError: mocks.showError, showInfo: vi.fn(), showWarning: vi.fn() }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ activeSubscriptions: [], fetchActiveSubscriptions: vi.fn() }) }))
vi.mock('@/stores/payment', () => ({ usePaymentStore: () => ({ createOrder: mocks.createOrder }) }))
vi.mock('@/api/invoices', () => ({ invoiceAPI: { quote: mocks.quote, get: mocks.get, create: mocks.createInvoice } }))
vi.mock('@/api/payment', () => ({ paymentAPI: { getCheckoutInfo: async () => ({ data: {
  methods: { wxpay: { currency: 'CNY', available: true, single_min: 0, single_max: 0 }, stripe: { currency: 'USD', available: true } },
  plans: [], global_min: 100, global_max: 10000, balance_disabled: true, balance_recharge_multiplier: 20, recharge_fee_rate: 15,
} }) } }))
vi.mock('@/utils/device', () => ({ isMobileDevice: () => false }))

const quote: InvoiceQuote = { orders: [{ id: 1, order_no: '000000123', order_type: 'balance', name: '', amount: 311.5 }], currency: 'CNY', base_amount: 311.5, service_fee: 38, total_amount: 349.5, net_amount: 339.32, tax_amount: 10.18, item_name: 'Technical services', tax_rate: 3, tier: { upper_amount: null, type: 'fixed', value: 38 }, fingerprint: 'quote-1' }
let wrapper: VueWrapper | undefined
beforeEach(() => {
  vi.clearAllMocks(); localStorage.clear()
  mocks.route.query = { selection: 'all' }
  mocks.quote.mockResolvedValue(quote)
  mocks.createInvoice.mockResolvedValue({ id: 77, status: 'awaiting_payment', quote })
  mocks.createOrder.mockResolvedValue({ order_id: 999, invoice_request_id: 77, amount: 38, pay_amount: 38, fee_rate: 0, currency: 'CNY', expires_at: '2099-01-01T00:00:00Z', qr_code: 'invoice-qr', payment_type: 'wxpay' })
  mocks.replace.mockResolvedValue(undefined)
})
afterEach(() => { wrapper?.unmount() })
async function open() {
  wrapper = mount(PaymentView, { props: { invoiceMode: true }, global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' }, Icon: true, RouterLink: RouterLinkStub,
    BaseDialog: { props: ['show'], template: '<div v-if="show" data-preview><slot /><slot name="footer" /></div>' },
    PaymentStatusPanel: { props: ['orderType', 'payAmount'], template: '<div data-payment>{{ orderType }} {{ payAmount }}</div>' },
  } } })
  await flushPromises(); return wrapper
}
async function fill(w: VueWrapper) {
  await w.get('#invoice-tax').setValue('001234567890')
  await w.get('#invoice-title').setValue('Example buyer')
  await w.get('#invoice-email').setValue('billing@example.com')
}
describe('invoice checkout using the real shared payment view', () => {
  it('previews an inclusive invoice and pays only the service fee when recharge/subscriptions are disabled', async () => {
    const w = await open()
    expect(mocks.quote).toHaveBeenCalledWith({ selection: 'all' })
    expect(w.find('input[type="number"]').exists()).toBe(false)
    await fill(w)
    await w.get('form').trigger('submit')
    expect(w.get('[data-preview]').text()).toContain('349.50')
    expect(w.get('[data-preview]').text()).toContain('339.32')
    expect(mocks.createInvoice).not.toHaveBeenCalled()
    await w.findAll('button').find(b => b.text().includes('invoices.pay'))!.trigger('click')
    await flushPromises()
    expect(mocks.createInvoice.mock.calls[0][0]).toMatchObject({ order_ids: [1], tax_id: '001234567890', title: 'Example buyer', email: 'billing@example.com', remarks: '', quote_fingerprint: 'quote-1' })
    expect(mocks.createOrder).toHaveBeenCalledWith(expect.objectContaining({ order_type: 'invoice_fee', invoice_request_id: 77, amount: 38, payment_type: 'wxpay' }))
    expect(w.get('[data-payment]').text()).toContain('invoice_fee 38')
    expect(JSON.parse(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)!)).toMatchObject({ orderType: 'invoice_fee', invoiceRequestId: 77, payAmount: 38 })
  })
  it('retains the application operation key when an uncertain create is retried', async () => {
    mocks.createInvoice.mockRejectedValueOnce(new Error('network timeout'))
    const w = await open(); await fill(w)
    await w.findAll('button').find(b => b.text().includes('invoices.pay'))!.trigger('click'); await flushPromises()
    expect(mocks.createOrder).not.toHaveBeenCalled()
    await w.findAll('button').find(b => b.text().includes('invoices.pay'))!.trigger('click'); await flushPromises()
    expect(mocks.createInvoice.mock.calls[0][1]).toBe(mocks.createInvoice.mock.calls[1][1])
    expect(mocks.createOrder).toHaveBeenCalledTimes(1)
  })
  it('does not submit empty required fields', async () => {
    const w = await open()
    await w.findAll('button').find(b => b.text().includes('invoices.pay'))!.trigger('click'); await flushPromises()
    expect(mocks.createInvoice).not.toHaveBeenCalled()
    expect(mocks.createOrder).not.toHaveBeenCalled()
  })
  it('does not reopen a provider URL when creation replays an already completed order', async () => {
    const openWindow = vi.spyOn(window, 'open').mockReturnValue(null)
    mocks.createOrder.mockResolvedValue({ order_id: 999, invoice_request_id: 77, amount: 38, pay_amount: 38, fee_rate: 0, currency: 'CNY', status: 'COMPLETED', payment_type: 'wxpay', pay_url: 'https://pay.example.com/already-paid', expires_at: '2099-01-01T00:00:00Z' })
    const w = await open(); await fill(w)
    await w.findAll('button').find(b => b.text().includes('invoices.pay'))!.trigger('click'); await flushPromises()
    expect(openWindow).not.toHaveBeenCalled()
    expect(w.get('[data-payment]').text()).toContain('invoice_fee 38')
    openWindow.mockRestore()
  })
  it('retains invoice identity through signed WeChat return parsing', () => {
    const query = { invoice_request_id: '77', order_type: 'invoice_fee', wechat_resume_token: 'signed', wechat_resume: '1' }
    expect(parseWechatResumeRoute(query, [], 100)).toMatchObject({ orderType: 'invoice_fee', invoiceRequestId: 77, orderAmount: 0, wechatResumeToken: 'signed' })
    expect(stripWechatResumeQuery(query)).toEqual({ invoice_request_id: '77' })
  })
})
