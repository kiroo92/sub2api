import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PackagePlansView from '../PackagePlansView.vue'
const { savePlan } = vi.hoisted(() => ({ savePlan: vi.fn().mockResolvedValue({}) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/packages', () => ({ packagesAPI: { adminPlans: vi.fn().mockResolvedValue([]), savePlan } }))
vi.mock('@/api/admin/groups', () => ({ default: {}, getAll: vi.fn().mockResolvedValue([{ id: 9, name: 'Models', platform: 'openai' }]) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
describe('new package administration', () => {
  it('defaults to unpublished, applies week/month group durations, and rejects a decreasing quota tier', async () => {
    const wrapper = mount(PackagePlansView, { global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
    } } })
    await flushPromises()
    await wrapper.get('button.btn-primary').trigger('click')
    const label = (key: string) => wrapper.findAll('label').find(item => item.text().includes(key))!
    await label('packages.name').get('input').setValue('Monthly')
    await label('packages.group').get('select').setValue('9')
    await label('packages.price').get('input').setValue('385')
    await label('packages.baseQuota').get('input').setValue('1800')
    await label('packages.cardType').get('select').setValue('30')
    await label('packages.groupEnabled').get('input').setValue(true)
    expect((label('packages.hours').get('input').element as HTMLInputElement).value).toBe('48')
    await wrapper.findAll('button').find(item => item.text() === 'packages.addTier')!.trigger('click')
    await label('packages.tierQuota').get('input').setValue('1600')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.text()).toContain('packages.invalidPlan')
    expect(savePlan).not.toHaveBeenCalled()
    await label('packages.tierQuota').get('input').setValue('1980')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(savePlan).toHaveBeenCalledWith(expect.objectContaining({ name: 'Monthly', group_id: 9, price: 385, currency: 'CNY', validity_days: 30, group_buy_hours: 48, for_sale: false, base_quota_usd: 1800, tiers: [{ members: 2, quota_usd: 1980 }] }))
    expect(wrapper.find('form').exists()).toBe(false)
    wrapper.unmount()
  })
})
