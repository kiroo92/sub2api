import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminDiscountCodesView from '../AdminDiscountCodesView.vue'

const { getDiscountCodes, getPlans, createDiscountCode, updateDiscountCode } = vi.hoisted(() => ({ getDiscountCodes: vi.fn(), getPlans: vi.fn(), createDiscountCode: vi.fn(), updateDiscountCode: vi.fn() }))
vi.mock('@/api/admin/payment', () => {
  const api = { getDiscountCodes, getPlans, createDiscountCode, updateDiscountCode }
  return { adminPaymentAPI: api, default: api }
})
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

function mountView() {
  return mount(AdminDiscountCodesView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
    RouterLink: { template: '<a><slot /></a>' },
  } } })
}

describe('subscription discount management', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getDiscountCodes.mockResolvedValue({ data: { items: [], total: 0 } })
    getPlans.mockResolvedValue({ data: [{ id: 7, name: 'Monthly' }] })
    createDiscountCode.mockResolvedValue({ data: {} })
    updateDiscountCode.mockResolvedValue({ data: {} })
  })

  it('saves eight-fold pricing as 80 percent payable with plan and usage restrictions', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="create-discount"]').trigger('click')
    await wrapper.get('#discount-code').setValue('vip80')
    await wrapper.get('#discount-value').setValue(8)
    await wrapper.get('fieldset input').setValue(true)
    await wrapper.get('#discount-total').setValue(20)
    await wrapper.get('#discount-form').trigger('submit.prevent')
    await flushPromises()
    expect(createDiscountCode).toHaveBeenCalledWith(expect.objectContaining({ code: 'VIP80', discount_type: 'percentage', discount_value: 80, plan_ids: [7], max_uses: 20, per_user_limit: 1, enabled: true }))
    wrapper.unmount()
  })

  it('keeps the editor open and shows server validation errors', async () => {
    createDiscountCode.mockRejectedValue({ message: 'Code already exists' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="create-discount"]').trigger('click')
    await wrapper.get('#discount-code').setValue('SAME')
    await wrapper.get('#discount-form').trigger('submit.prevent')
    await flushPromises()
    expect(wrapper.find('#discount-form').exists()).toBe(true)
    expect(wrapper.text()).toContain('Code already exists')
    wrapper.unmount()
  })
})
