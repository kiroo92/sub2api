import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import OwnedPackages from '@/components/packages/OwnedPackages.vue'
import PackageHoldingCard from '@/components/packages/PackageHoldingCard.vue'
import PackageShop from '@/components/packages/PackageShop.vue'
import PackagePaymentDialog from '@/components/packages/PackagePaymentDialog.vue'
import SubscriptionsView from '../SubscriptionsView.vue'
import PackageGroupsView from '../PackageGroupsView.vue'
import type { PackageGroupBuy, PackagePlan, UserPackage } from '@/types/packages'
import { resolvePackageThemeColor } from '@/types/packages'
import en from '@/i18n/locales/en'
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string, params: Record<string, string | number> = {}) => {
  const message = key.split('.').reduce<unknown>((value, part) => value && typeof value === 'object' ? (value as Record<string, unknown>)[part] : undefined, en)
  return typeof message === 'string' ? message.replace(/\{(\w+)\}/g, (_, param: string) => String(params[param] ?? '')) : key
} }) }))
const mocks = vi.hoisted(() => ({ mine: vi.fn(), reorder: vi.fn(), legacy: vi.fn(), groups: vi.fn(), group: vi.fn(), plans: vi.fn(), startGroup: vi.fn(), push: vi.fn(), showError: vi.fn(), params: {} as { id?: string } }))
vi.mock('@/api/packages', () => ({ packagesAPI: { mine: mocks.mine, reorder: mocks.reorder, groups: mocks.groups, group: mocks.group, plans: mocks.plans, startGroup: mocks.startGroup } }))
vi.mock('@/api/subscriptions', () => ({ default: { getMySubscriptions: mocks.legacy } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: mocks.showError }) }))
vi.mock('vue-router', async () => ({ ...await vi.importActual<typeof import('vue-router')>('vue-router'), useRoute: () => ({ params: mocks.params }), useRouter: () => ({ push: mocks.push }) }))
const plan: PackagePlan = { id: 1, name: 'Month card', group_id: 10, group_name: 'Models', group_platform: 'openai', description: '', validity_days: 30, base_quota_usd: 1800, price: 385, currency: 'CNY', group_buy_enabled: true, group_buy_hours: 48, tiers: [{ members: 3, quota_usd: 1980 }, { members: 5, quota_usd: 2100 }, { members: 10, quota_usd: 2400 }], theme_color: 'violet', for_sale: true, sort_order: 0 }
function holding(id: number, expired = false): UserPackage {
  const periods = [{ id: id * 10, package_id: id, period_index: 1, starts_at: '2026-09-01T00:00:00Z', ends_at: '2026-09-08T00:00:00Z', quota_usd: 450, used_usd: 450 }, { id: id * 10 + 1, package_id: id, period_index: 2, starts_at: '2026-09-08T00:00:00Z', ends_at: '2026-09-15T00:00:00Z', quota_usd: 450, used_usd: 10 }]
  return { id, user_id: 1, group_id: 10, order_id: id, plan, status: 'active', sort_order: id, group_buy_id: null, starts_at: '2026-09-01T00:00:00Z', expires_at: expired ? '2026-09-02T00:00:00Z' : '2026-10-01T00:00:00Z', periods, current_period: periods[1] }
}
function group(id: number): PackageGroupBuy {
  return { id, plan: { ...plan, name: `Group ${id} card` }, status: 'open', paid_count: 2, target_members: 10, starts_at: '2026-09-09T00:00:00Z', ends_at: '2026-09-11T00:00:00Z', settled_at: null, final_members: null, final_quota_usd: null, joined: false, order_id: null }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
function options() {
  return { global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      PackagePaymentDialog: true,
      RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      VueDraggable: { name: 'VueDraggable', props: ['modelValue', 'disabled'], template: '<div data-test="sortable"><slot /></div>' },
    },
  } }
}
describe('new package holdings and group display', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-10T00:00:00Z'))
    mocks.mine.mockReset().mockResolvedValue([holding(1), holding(2), holding(3, true)])
    mocks.reorder.mockReset().mockResolvedValue(undefined)
    mocks.legacy.mockResolvedValue([{ id: 1, group_id: 10, status: 'active', group: { name: 'Old subscription', platform: 'openai', daily_limit_usd: 10 }, daily_usage_usd: 2, expires_at: '2099-01-01T00:00:00Z' }])
    mocks.groups.mockReset().mockResolvedValue([group(1)])
    mocks.group.mockReset().mockImplementation(async (id: number) => group(id))
    mocks.plans.mockReset().mockResolvedValue([plan])
    mocks.startGroup.mockReset().mockResolvedValue(group(5))
    mocks.params = reactive({})
  })
  afterEach(() => { vi.useRealTimers() })
  it('uses a deterministic palette fallback for old plan snapshots', () => {
    expect(resolvePackageThemeColor(undefined, 9)).toBe('emerald')
    expect(resolvePackageThemeColor('not-a-preset', 9)).toBe('emerald')
  })
  it('only reorders active new packages and rolls back a failed save', async () => {
    const wrapper = mount(OwnedPackages, options())
    await flushPromises()
    expect(wrapper.findAll('[data-test="sortable"] [data-package-id]')).toHaveLength(2)
    expect(wrapper.findAll('details [data-package-id]')).toHaveLength(1)
    mocks.reorder.mockRejectedValueOnce(new Error('conflict'))
    mocks.mine.mockRejectedValueOnce(new Error('offline'))
    await wrapper.get('button[aria-label="Move Month card down"]').trigger('click')
    await flushPromises()
    expect(mocks.reorder).toHaveBeenCalledWith([2, 1])
    expect(wrapper.findAll('[data-test="sortable"] [data-package-id]').map(item => item.attributes('data-package-id'))).toEqual(['1', '2'])
    expect(wrapper.get('[role="alert"]').text()).toContain('conflict')
    wrapper.unmount()
  })
  it('combines display without giving legacy records reorder or renewal controls', async () => {
    const wrapper = mount(SubscriptionsView, options())
    await flushPromises()
    expect(wrapper.text()).toContain('Legacy package')
    expect(wrapper.text()).toContain('Old subscription')
    expect(wrapper.text()).toContain('Month card')
    expect(wrapper.text()).toContain('$2.00 / $10.00')
    expect(wrapper.text()).not.toContain('Renew Now')
    expect(wrapper.findAll('button[aria-label^="Move "]')).toHaveLength(4)
    wrapper.unmount()
  })
  it('shows settled quota and disables joining ended groups', async () => {
    mocks.params = { id: '5' }
    mocks.group.mockResolvedValue({ id: 5, plan, status: 'settled', paid_count: 8, target_members: 10, starts_at: '2026-09-07T00:00:00Z', ends_at: '2026-09-09T00:00:00Z', final_members: 8, final_quota_usd: 2100, joined: false, order_id: null })
    const wrapper = mount(PackageGroupsView, { ...options(), global: { ...options().global, stubs: { ...options().global.stubs, PackageRules: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('$2,100')
    expect(wrapper.text()).not.toContain('00:00:00 remaining')
    expect(wrapper.text()).not.toContain('Next tier quota')
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it.each([false, true])('shows existing payment access (paid=%s) without a second participation', async (joined) => {
    mocks.params = { id: '5' }
    mocks.group.mockResolvedValue({ id: 5, plan, status: 'open', paid_count: 1, target_members: 10, ends_at: '2099-01-01T00:00:00Z', joined, order_id: 7 })
    const wrapper = mount(PackageGroupsView, options())
    await flushPromises()
    if (joined) {
      const link = wrapper.findAll('a').find(item => item.text() === 'View existing order')!
      expect(link.attributes('href')).toBe('/orders')
      expect(wrapper.get('.package-detail__purchased').text()).toContain('Base package activated')
      expect(wrapper.get('.package-detail__purchased').text()).toContain('Group rewards will be settled when this group ends')
      expect(wrapper.get('a[href="/dashboard#subscriptions"]').text()).toContain('View my subscriptions')
    } else {
      await wrapper.findAll('button').find(item => item.text() === 'Continue payment')!.trigger('click')
      expect(wrapper.getComponent(PackagePaymentDialog).props('group').order_id).toBe(7)
      expect(mocks.push).not.toHaveBeenCalled()
    }
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('shows fixed draft duration and product facts before the bottom purchase action', async () => {
    mocks.params.id = '5'
    mocks.group.mockResolvedValue({ ...group(5), status: 'draft', paid_count: 0, starts_at: null, ends_at: null })
    const wrapper = mount(PackageGroupsView, options())
    await flushPromises()
    expect(wrapper.get('.package-detail__timer').text()).toBe('48:00:00')
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.get('.package-detail__timer').text()).toBe('48:00:00')
    expect(wrapper.get('.package-detail__countdown').text()).toContain('Starts when the initiator pays')
    expect(wrapper.get('.package-detail__facts').text()).toContain('$450')
    expect(wrapper.get('.package-detail__rules').text()).toContain('Join any number of different groups')
    const grid = wrapper.get('.package-detail__grid').element
    const footer = wrapper.get('.package-detail__action').element
    expect(grid.nextElementSibling).toBe(footer)
    expect(footer.querySelector('input[type="checkbox"]')).not.toBeNull()
    expect(wrapper.get('.package-detail__buy').text()).toBe('Buy and start group')
    expect(wrapper.get('.package-detail__buy').attributes('disabled')).toBeDefined()
    expect(wrapper.find('.package-detail__tiers button, .package-detail__tiers input').exists()).toBe(false)
    expect(wrapper.get('.package-detail__quota-summary').text()).toContain('$1,800')
    wrapper.unmount()
  })
  it('ends new participation at the live deadline while retaining the settled-tier display', async () => {
    mocks.params.id = '5'
    mocks.group.mockResolvedValue({ ...group(5), paid_count: 8, ends_at: '2026-09-10T00:00:02Z' })
    const wrapper = mount(PackageGroupsView, options())
    await flushPromises()
    expect(wrapper.get('.package-detail__timer').text()).toBe('00:00:02')
    expect(wrapper.get('.package-detail__tier.is-current').text()).toContain('5-person group')
    expect(wrapper.get('.package-detail__quota-summary').text()).toContain('$2,100')
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(false)
    expect(wrapper.find('.package-detail__buy').exists()).toBe(false)
    expect(wrapper.get('.package-detail__status').text()).toBe('Awaiting settlement')
    wrapper.unmount()
  })
  it('shows frozen settlement and delivered membership without another purchase', async () => {
    mocks.params.id = '5'
    mocks.group.mockResolvedValue({ ...group(5), status: 'settled', joined: true, order_id: 7, paid_count: 10, final_members: 8, final_quota_usd: 2100 })
    const wrapper = mount(PackageGroupsView, options())
    await flushPromises()
    expect(wrapper.get('.package-detail__progress-label').text()).toContain('8/10 people')
    expect(wrapper.get('.package-detail__tier.is-current').text()).toContain('5-person group')
    expect(wrapper.get('.package-detail__quota-summary').text()).toContain('Settled quota$2,100')
    expect(wrapper.get('.package-detail__purchased').text()).toContain('Package quotas now reflect the final tier')
    expect(wrapper.find('.package-detail__countdown').exists()).toBe(false)
    expect(wrapper.find('.package-detail__action button').exists()).toBe(false)
    wrapper.unmount()
  })
  it('does not display a fifth reset at day 28 of a month card', () => {
    const item = holding(1)
    item.periods = [{ id: 14, package_id: 1, period_index: 4, starts_at: '2026-09-22T00:00:00Z', ends_at: '2026-10-01T00:00:00Z', quota_usd: 450, used_usd: 12 }]
    vi.setSystemTime(new Date('2026-09-29T00:00:00Z'))
    const wrapper = mount(PackageHoldingCard, { ...options(), props: { item } })
    expect(wrapper.text()).toContain('$12.00 used / $450.00')
    expect(wrapper.text()).toContain('Expires at the end of this period')
    wrapper.unmount()
  })
  it('only shows the latest route while an older detail request is pending', async () => {
    const first = deferred<PackageGroupBuy>()
    const second = deferred<PackageGroupBuy>()
    mocks.params.id = '1'
    mocks.group.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const wrapper = mount(PackageGroupsView, options())
    mocks.params.id = '2'
    await flushPromises()
    expect(mocks.group).toHaveBeenLastCalledWith(2)
    second.resolve(group(2))
    await flushPromises()
    expect(wrapper.text()).toContain('Group 2 card')
    first.resolve(group(1))
    await flushPromises()
    expect(wrapper.text()).not.toContain('Group 1 card')
    expect(wrapper.text()).toContain('Group 2 card')
    wrapper.unmount()
  })
  it('does not let a stale hall failure replace the current detail', async () => {
    const hall = deferred<PackageGroupBuy[]>()
    mocks.groups.mockReturnValueOnce(hall.promise)
    const wrapper = mount(PackageGroupsView, options())
    mocks.params.id = '2'
    await flushPromises()
    hall.reject(new Error('old hall error'))
    await flushPromises()
    expect(wrapper.text()).toContain('Group 2 card')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('retains loaded groups through a poll failure and supports retry', async () => {
    const wrapper = mount(PackageGroupsView, options())
    await flushPromises()
    mocks.groups.mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(wrapper.text()).toContain('Group 1 card')
    expect(wrapper.get('[role="alert"]').text()).toContain('offline')
    await wrapper.get('button[aria-label="Refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('a[href="/package-groups/1"]').text()).toContain('Group buy details')
    wrapper.unmount()
  })
  it('keeps one accent per plan while preserving the API deadline order', async () => {
    const later = { ...group(1), ends_at: '2026-09-12T00:00:00Z', plan: { ...plan, id: 1, name: 'Month card', theme_color: 'violet' as const } }
    const earlier = { ...group(2), ends_at: '2026-09-11T00:00:00Z', plan: { ...plan, id: 2, name: 'Week card', validity_days: 7 as const, theme_color: 'emerald' as const } }
    const samePlan = { ...group(3), ends_at: '2026-09-13T00:00:00Z', plan: { ...plan, id: 1, name: 'Month card', theme_color: 'violet' as const } }
    mocks.groups.mockResolvedValueOnce([later, earlier, samePlan])
    const wrapper = mount(PackageGroupsView, options())
    await flushPromises()
    const cards = wrapper.findAll('[data-theme]')
    expect(cards).toHaveLength(3)
    expect(cards.map(card => card.attributes('data-theme'))).toEqual(['violet', 'emerald', 'violet'])
    expect(cards.map(card => card.get('h2').text())).toEqual(['Month card', 'Week card', 'Month card'])
    expect(cards[0].attributes('style')).toContain('--package-accent')
    wrapper.unmount()
  })
  it('resets consent on detail navigation and provides a hall link', async () => {
    mocks.params.id = '1'
    const wrapper = mount(PackageGroupsView, options())
    await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    mocks.params.id = '2'
    await flushPromises()
    expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.get('a[href="/package-groups"]').text()).toContain('Back to group buys')
    const join = wrapper.findAll('button').find(button => button.text().includes('Buy and join group'))!
    expect(join.attributes('disabled')).toBeDefined()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await join.trigger('click')
    expect(mocks.push).not.toHaveBeenCalled()
    expect(wrapper.getComponent(PackagePaymentDialog).props()).toMatchObject({ show: true, termsAccepted: true, group: { id: 2 } })
    wrapper.unmount()
  })
  it('locks ordering throughout a refresh and ignores an unchanged drag', async () => {
    const wrapper = mount(OwnedPackages, options())
    await flushPromises()
    wrapper.getComponent({ name: 'VueDraggable' }).vm.$emit('end')
    await flushPromises()
    expect(mocks.reorder).not.toHaveBeenCalled()
    const refresh = deferred<UserPackage[]>()
    mocks.mine.mockReturnValueOnce(refresh.promise)
    await vi.advanceTimersByTimeAsync(30000)
    const move = wrapper.get('button[aria-label="Move Month card down"]')
    expect(move.attributes('disabled')).toBeDefined()
    await move.trigger('click')
    expect(mocks.reorder).not.toHaveBeenCalled()
    refresh.resolve([holding(1), holding(2)])
    await flushPromises()
    expect(move.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('keeps the saved order as rollback state if its follow-up refresh fails', async () => {
    const wrapper = mount(OwnedPackages, options())
    await flushPromises()
    mocks.mine.mockRejectedValue(new Error('offline'))
    await wrapper.get('button[aria-label="Move Month card down"]').trigger('click')
    await flushPromises()
    mocks.reorder.mockRejectedValueOnce(new Error('conflict'))
    await wrapper.get('button[aria-label="Move Month card down"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-test="sortable"] [data-package-id]').map(item => item.attributes('data-package-id'))).toEqual(['2', '1'])
    expect(wrapper.get('[role="alert"]').text()).toContain('conflict')
    wrapper.unmount()
  })
  it('does not show an empty subscription state during package loading or failure', async () => {
    const pending = deferred<UserPackage[]>()
    mocks.mine.mockReturnValueOnce(pending.promise)
    mocks.legacy.mockResolvedValue([])
    const wrapper = mount(SubscriptionsView, options())
    await flushPromises()
    expect(wrapper.text()).not.toContain('No Active Subscriptions')
    expect(wrapper.text()).not.toContain('No subscription packages yet')
    pending.reject(new Error('offline'))
    await flushPromises()
    expect(wrapper.text()).not.toContain('No Active Subscriptions')
    expect(wrapper.text()).not.toContain('No subscription packages yet')
    wrapper.unmount()
  })
  it('shows one combined subscription empty state with a shop action', async () => {
    mocks.mine.mockResolvedValue([])
    mocks.legacy.mockResolvedValue([])
    const wrapper = mount(SubscriptionsView, options())
    await flushPromises()
    expect(wrapper.text().match(/No subscription packages yet/g)).toHaveLength(1)
    expect(wrapper.text()).not.toContain('No new packages yet')
    expect(wrapper.text()).not.toContain('No Active Subscriptions')
    expect(wrapper.get('a[href="/purchase?tab=subscription"]').text()).toContain('Package shop')
    wrapper.unmount()
  })
  it('offers retry instead of an empty state when legacy holdings fail to load', async () => {
    mocks.mine.mockResolvedValue([])
    mocks.legacy.mockRejectedValueOnce(new Error('offline')).mockResolvedValue([])
    const wrapper = mount(SubscriptionsView, options())
    await flushPromises()
    expect(wrapper.text()).not.toContain('No subscription packages yet')
    expect(wrapper.get('[role="alert"]').text()).toContain('Failed to load subscriptions')
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('No subscription packages yet')
    wrapper.unmount()
  })
  it('selects a single plan and recovers from a failed, duplicate group start', async () => {
    const wrapper = mount(PackageShop, options())
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'Buy individually')!.trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual([plan])
    const pending = deferred<PackageGroupBuy>()
    mocks.startGroup.mockReturnValueOnce(pending.promise)
    const start = wrapper.findAll('button').find(button => button.text() === 'Start group')!
    const first = start.trigger('click')
    const duplicate = start.trigger('click')
    await Promise.all([first, duplicate])
    expect(mocks.startGroup).toHaveBeenCalledTimes(1)
    pending.reject(new Error('offline'))
    await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith('offline')
    expect(start.attributes('disabled')).toBeUndefined()
    await start.trigger('click')
    await flushPromises()
    expect(mocks.push).not.toHaveBeenCalled()
    expect(wrapper.emitted('start')?.[0]).toEqual([plan, group(5)])
    await start.trigger('click')
    await flushPromises()
    expect(mocks.startGroup).toHaveBeenCalledTimes(3)
    expect(mocks.group).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
