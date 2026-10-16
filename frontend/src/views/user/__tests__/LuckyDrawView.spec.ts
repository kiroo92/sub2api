import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LuckyDrawView from '../LuckyDrawView.vue'
import AdminLuckyDrawView from '@/views/admin/LuckyDrawView.vue'
import type { LuckyDrawSnapshot } from '@/api/luckyDraw'

const { get, draw, adminGet, configure, refreshUser, stats, drawBatch, grant } = vi.hoisted(() => ({ get: vi.fn(), draw: vi.fn(), adminGet: vi.fn(), configure: vi.fn(), refreshUser: vi.fn(), stats: vi.fn(), drawBatch: vi.fn(), grant: vi.fn() }))
vi.mock('@/api/luckyDraw', () => ({ luckyDrawAPI: { get, draw, adminGet, configure, stats, drawBatch, grant } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7 }, refreshUser }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }) }))
const fixture = (): LuckyDrawSnapshot => ({
  config: { enabled: true, version: 2, recharge_starts_at: '2026-10-15T16:00:00Z', recharge_per_draw: 5, guarantee_after: 3, prizes: [{ name: '$1', amount: 1, weight: 100 }, { name: 'No prize', amount: 0, weight: 9900 }] },
  manual_chances: 0, chances: 1, draws_used: 1, total_recharged: 100, misses: 1, history: []
})
const award = { id: 1, request_id: 'd66d1ae0-9bea-4567-8ab7-9198e788cb6f', config_version: 2, prize_index: 0, prize_name: '$1', prize_amount: 1, guaranteed: false, created_at: '2026-10-15T00:00:00Z' }
const wrappers: VueWrapper[] = []
function page(admin = false) {
  const wrapper = mount(admin ? AdminLuckyDrawView : LuckyDrawView, { global: { stubs: { AppLayout: defineComponent({ template: '<main><slot /></main>' }), RouterLink: true } } })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks(); localStorage.clear(); vi.useFakeTimers()
  stats.mockResolvedValue({ recharge_starts_at: '2026-10-15T16:00:00Z', summary: { recharge_amount: 10, manual_chances: 0, chances_earned: 2, draws_used: 1, chances: 1, wins: 1, prize_amount: 0.5 }, users: [{ user_id: 7, email: 'lucky@example.com', recharge_amount: 10, manual_chances: 0, chances_earned: 2, draws_used: 1, chances: 1, wins: 1, prize_amount: 0.5 }], grants: [], prizes: [{ name: '¥0.5', amount: 0.5, draws: 1, total_awarded: 0.5 }], total: 1, page: 1, page_size: 20 })
  get.mockResolvedValue(fixture()); adminGet.mockResolvedValue(fixture()); configure.mockResolvedValue(fixture().config); refreshUser.mockResolvedValue({})
})
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.useRealTimers() })

describe('Lucky draw', () => {
  it.each(['paused', 'empty'])('prevents draws when %s', async state => {
    const data = fixture()
    if (state === 'paused') data.config.enabled = false
    else data.chances = 0
    get.mockResolvedValue(data)
    const wrapper = page(); await flushPromises()
    expect(wrapper.get('.lucky-card').attributes('disabled')).toBeDefined()
    expect(draw).not.toHaveBeenCalled()
  })
  it('deduplicates clicks and refreshes the credited balance', async () => {
    let resolve!: (value: unknown) => void
    draw.mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = page(); await flushPromises()
    await wrapper.get('.lucky-card').trigger('click'); await wrapper.get('.lucky-card').trigger('click')
    expect(draw).toHaveBeenCalledTimes(1)
    expect(draw.mock.calls[0][0]).toMatchObject({ config_version: 2 })
    expect(localStorage.getItem('lucky-draw-pending:7')).toContain(draw.mock.calls[0][0].request_id)
    resolve(award); await flushPromises(); await vi.advanceTimersByTimeAsync(650); await flushPromises()
    expect(wrapper.text()).toContain('luckyDraw.won')
    expect(localStorage.getItem('lucky-draw-pending:7')).toBeNull()
    expect(refreshUser).toHaveBeenCalledTimes(1)
  })
  it('persists the same request across an uncertain result and page reload, even with no chances', async () => {
    draw.mockRejectedValueOnce({ status: 504 })
    const first = page(); await flushPromises()
    await first.get('.lucky-card').trigger('click'); await flushPromises()
    const pending = draw.mock.calls[0][0]
    expect(first.text()).toContain('luckyDraw.drawFailed')
    first.unmount(); wrappers.splice(wrappers.indexOf(first), 1)
    get.mockResolvedValue({ ...fixture(), chances: 0 })
    draw.mockResolvedValue(award)
    const second = page(); await flushPromises()
    const recover = second.findAll('button').find(b => b.text() === 'luckyDraw.recover')!
    await recover.trigger('click'); await flushPromises(); await vi.advanceTimersByTimeAsync(650); await flushPromises()
    expect(draw.mock.calls[1][0]).toEqual(pending)
    expect(second.text()).toContain('luckyDraw.won')
  })
  it('discards a definitely rejected stale request without automatically drawing again', async () => {
    draw.mockRejectedValue({ reason: 'LUCKY_DRAW_CHANGED' })
    const wrapper = page(); await flushPromises()
    await wrapper.get('.lucky-card').trigger('click'); await flushPromises()
    expect(localStorage.getItem('lucky-draw-pending:7')).toBeNull()
    expect(draw).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('luckyDraw.changed')
  })
  it('clears a recovered old prize when switching to a new pool presentation', async () => {
    localStorage.setItem('lucky-draw-pending:7', JSON.stringify({ request_id: award.request_id, config_version: 1 }))
    draw.mockResolvedValue({ ...award, config_version: 1, prize_index: 8 })
    const wrapper = page(); await flushPromises()
    await wrapper.get('.lucky-card').trigger('click'); await flushPromises(); await vi.advanceTimersByTimeAsync(650); await flushPromises()
    expect(wrapper.text()).toContain('luckyDraw.won')
    await wrapper.findAll('button').find(b => b.text() === 'luckyDraw.wheel')!.trigger('click')
    expect(wrapper.text()).not.toContain('luckyDraw.won')
    expect(draw).toHaveBeenCalledTimes(1)
  })
  it.each(['wheel', 'roller'])('reveals the committed server result in %s mode', async mode => {
    draw.mockResolvedValue(award)
    const wrapper = page(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === `luckyDraw.${mode}`)!.trigger('click')
    await wrapper.get(mode === 'wheel' ? '.wheel-button' : '.roller button').trigger('click')
    await flushPromises(); await vi.advanceTimersByTimeAsync(1950); await flushPromises()
    expect(draw).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('luckyDraw.won')
    const style = wrapper.get(mode === 'wheel' ? '.wheel' : '.roller-strip').attributes('style')
    expect(style).toContain(mode === 'wheel' ? 'rotate(1710deg)' : 'translateY(-720px)')
  })
  it('saves admin rules with the loaded version', async () => {
    const wrapper = page(true); await flushPromises()
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(configure).toHaveBeenCalledWith(fixture().config)
    expect(wrapper.text()).toContain('luckyDraw.saved')
  })
  it('shows statistics inside draw management and searches by user', async () => {
    const wrapper = page(true); await flushPromises()
    expect(wrapper.text()).toContain('luckyDraw.statsTitle')
    expect(wrapper.text()).toContain('lucky@example.com')
    expect(wrapper.text()).toContain('luckyDraw.costEstimate')
    await wrapper.get('input[aria-label="luckyDraw.searchUsers"]').setValue('lucky@example.com')
    await wrapper.findAll('form')[2]!.trigger('submit'); await flushPromises()
    expect(stats).toHaveBeenLastCalledWith('lucky@example.com', 1)
  })
  it.each([[2, 'rare'], [5, 'epic'], [20, 'legendary']])('reveals the rarity effect for %s', async (amount, rarity) => {
    draw.mockResolvedValue({ ...award, prize_amount: amount })
    const wrapper = page(); await flushPromises()
    await wrapper.get('.lucky-card').trigger('click'); await flushPromises(); await vi.advanceTimersByTimeAsync(650); await flushPromises()
    expect(wrapper.get('.reward-reveal').attributes('data-rarity')).toBe(rarity)
    expect(wrapper.find('.reward-sparks').exists()).toBe(true)
  })
  it('submits ten draws once and shows the committed batch in a result dialog', async () => {
    get.mockResolvedValue({ ...fixture(), chances: 10 })
    drawBatch.mockResolvedValue({ request_id: award.request_id, records: Array.from({ length: 10 }, (_, i) => ({ ...award, id: i + 1, prize_amount: i === 6 ? 20 : 0 })), total_awarded: 20 })
    const wrapper = page(); await flushPromises()
    await wrapper.findAll('.count-button')[2]!.trigger('click')
    await wrapper.get('.draw-button').trigger('click'); await flushPromises(); await vi.advanceTimersByTimeAsync(650); await flushPromises()
    expect(drawBatch).toHaveBeenCalledTimes(1)
    expect(drawBatch.mock.calls[0][0]).toMatchObject({ count: 10, config_version: 2 })
    expect(draw).not.toHaveBeenCalled()
    expect(document.body.querySelectorAll('[role="dialog"] .modal-result')).toHaveLength(10)
    expect(document.body.querySelector('.modal-total')?.textContent).toContain('20.00')
    expect(document.body.querySelector('.draw-result-modal')?.classList.contains('legendary')).toBe(true)
  })
  it('allows demo ten draws with zero chances without calling draw APIs or changing history', async () => {
    get.mockResolvedValue({ ...fixture(), chances: 0 })
    const wrapper = page(); await flushPromises()
    await wrapper.get('.demo-switch input').setValue(true)
    await wrapper.findAll('.count-button')[2]!.trigger('click')
    await wrapper.get('.draw-button').trigger('click'); await flushPromises(); await vi.advanceTimersByTimeAsync(650); await flushPromises()
    expect(draw).not.toHaveBeenCalled(); expect(drawBatch).not.toHaveBeenCalled()
    expect(refreshUser).not.toHaveBeenCalled(); expect(get).toHaveBeenCalledTimes(1)
    expect(document.body.querySelectorAll('[role="dialog"] .modal-result')).toHaveLength(10)
    expect(document.body.querySelector('.modal-receipt')?.textContent).toContain('luckyDraw.demoComplete')
    expect(localStorage.getItem('lucky-draw-pending:7')).toBeNull()
  })
  it('keeps ten-draw request and count across uncertain responses', async () => {
    get.mockResolvedValue({ ...fixture(), chances: 10 })
    drawBatch.mockRejectedValueOnce({ status: 504 })
    const first = page(); await flushPromises()
    await first.findAll('.count-button')[2]!.trigger('click')
    await first.get('.draw-button').trigger('click'); await flushPromises()
    const request = drawBatch.mock.calls[0][0]
    first.unmount(); wrappers.splice(wrappers.indexOf(first), 1)
    get.mockResolvedValue({ ...fixture(), chances: 0 })
    drawBatch.mockResolvedValue({ records: Array.from({ length: 10 }, (_, i) => ({ ...award, id: i + 1 })), total_awarded: 10 })
    const second = page(); await flushPromises()
    await second.findAll('button').find(b => b.text() === 'luckyDraw.recover')!.trigger('click'); await flushPromises(); await vi.advanceTimersByTimeAsync(650); await flushPromises()
    expect(drawBatch.mock.calls[1][0]).toEqual(request)
    expect(request.count).toBe(10)
  })
  it('retries a manual grant with the same idempotency key', async () => {
    grant.mockRejectedValueOnce({ status: 504 }).mockResolvedValueOnce({ user_id: 7, chances: 10 })
    const wrapper = page(true); await flushPromises()
    const form = wrapper.findAll('form')[1]!
    await form.get('input[type="number"]').setValue('7')
    await form.trigger('submit'); await flushPromises()
    const request = grant.mock.calls[0][0]
    expect(localStorage.getItem('lucky-draw-grant:7')).toContain(request.request_id)
    await form.trigger('submit'); await flushPromises()
    expect(grant.mock.calls[1][0]).toEqual(request)
    expect(localStorage.getItem('lucky-draw-grant:7')).toBeNull()
    expect(wrapper.text()).toContain('luckyDraw.granted')
  })
})
