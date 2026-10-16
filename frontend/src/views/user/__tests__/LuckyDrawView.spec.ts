import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LuckyDrawView from '../LuckyDrawView.vue'
import AdminLuckyDrawView from '@/views/admin/LuckyDrawView.vue'
import type { LuckyDrawSnapshot } from '@/api/luckyDraw'

const { get, draw, adminGet, configure, refreshUser } = vi.hoisted(() => ({ get: vi.fn(), draw: vi.fn(), adminGet: vi.fn(), configure: vi.fn(), refreshUser: vi.fn() }))
vi.mock('@/api/luckyDraw', () => ({ luckyDrawAPI: { get, draw, adminGet, configure } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7 }, refreshUser }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }) }))
const fixture = (): LuckyDrawSnapshot => ({
  config: { enabled: true, version: 2, recharge_per_draw: 50, guarantee_after: 3, prizes: [{ name: '$1', amount: 1, weight: 100 }, { name: 'No prize', amount: 0, weight: 9900 }] },
  chances: 1, draws_used: 1, total_recharged: 100, misses: 1, history: []
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
})
