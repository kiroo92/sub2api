import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LotteryView from '../LotteryView.vue'
import AdminLotteryView from '@/views/admin/LotteryView.vue'
import type { LotterySnapshot } from '@/api/lottery'

const { get, join, adminGet, configure, refreshUser, resetCaptcha } = vi.hoisted(() => ({ get: vi.fn(), join: vi.fn(), adminGet: vi.fn(), configure: vi.fn(), refreshUser: vi.fn(), resetCaptcha: vi.fn() }))
vi.mock('@/components/TurnstileWidget.vue', () => ({ default: defineComponent({ emits: ['verify', 'expire', 'error'], setup(_, { expose, emit }) { expose({ reset: resetCaptcha }); return () => h('button', { 'data-testid': 'verify-turnstile', onClick: () => emit('verify', 'turnstile-proof') }, 'Verify') } }) }))
vi.mock('@/api/lottery', () => ({ lotteryAPI: { get, join, adminGet, configure } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }) }))
const config = { turnstile_site_key: 'site-key', turnstile_secret_configured: true, enabled: true, prize_amount: 5, winner_count: 6, participant_target: 60, min_recharge: 50 }
const fixture = (): LotterySnapshot => ({
  config: { ...config },
  current: { ...config, id: 131, participant_count: 11, winners_drawn: 0, status: 'open', created_at: '2026-09-08T00:00:00Z', drawn_at: null },
  joined: false, eligible: true, total_recharged: 50, recent_winners: [], my_wins: [], recent_rounds: []
})
const wrappers: VueWrapper[] = []
const mountPage = (admin = false) => {
  const wrapper = mount(admin ? AdminLotteryView : LotteryView, { global: { stubs: { AppLayout: defineComponent({ template: '<main><slot /></main>' }), RouterLink: true } } })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => { vi.clearAllMocks(); get.mockResolvedValue(fixture()); adminGet.mockResolvedValue(fixture()); configure.mockResolvedValue(config); refreshUser.mockResolvedValue({}) })
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })

describe('Lottery participation', () => {
  it('blocks participation until Turnstile returns a token', async () => {
    const wrapper = mountPage(); await flushPromises()
    expect(wrapper.get('[data-testid="lottery-join"]').attributes('disabled')).toBeDefined()
    expect(join).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="verify-turnstile"]').trigger('click')
    expect(wrapper.get('[data-testid="lottery-join"]').attributes('disabled')).toBeUndefined()
  })
  it('blocks entry if dedicated credentials are missing', async () => {
    const data = fixture(); data.config.turnstile_secret_configured = false; get.mockResolvedValue(data)
    const wrapper = mountPage(); await flushPromises()
    expect(wrapper.get('[data-testid="lottery-join"]').attributes('disabled')).toBeDefined()
  })
  it('loads progress and submits only the displayed round once while pending', async () => {
    let resolve!: (value: unknown) => void
    join.mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = mountPage(); await flushPromises()
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('11')
    await wrapper.get('[data-testid="verify-turnstile"]').trigger('click')
    const button = wrapper.get('[data-testid="lottery-join"]')
    await button.trigger('click'); await button.trigger('click')
    expect(join).toHaveBeenCalledTimes(1); expect(join).toHaveBeenCalledWith(131, { turnstile_token: 'turnstile-proof' })
    get.mockResolvedValue({ ...fixture(), joined: true })
    resolve({ round_id: 131, drawn: false, already_joined: false }); await flushPromises()
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('lottery.participating')
  })
  it.each(['paused', 'ineligible', 'joined', 'noRound'])('blocks entry when %s', async state => {
    const data = fixture()
    if (state === 'paused') data.config.enabled = false
    if (state === 'ineligible') data.eligible = false
    if (state === 'joined') data.joined = true
    if (state === 'noRound') data.current = null
    get.mockResolvedValue(data)
    const wrapper = mountPage(); await flushPromises()
    expect(wrapper.get('[data-testid="lottery-join"]').attributes('disabled')).toBeDefined()
    expect(join).not.toHaveBeenCalled()
  })
  it('recovers a stale round without automatically joining the next one', async () => {
    join.mockRejectedValue({ reason: 'LOTTERY_ROUND_CHANGED' })
    const wrapper = mountPage(); await flushPromises()
    const next = fixture(); next.current!.id = 132
    get.mockResolvedValue(next)
    await wrapper.get('[data-testid="verify-turnstile"]').trigger('click'); await wrapper.get('[data-testid="lottery-join"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('lottery.roundChanged')
    expect(wrapper.text()).toContain('#132'); expect(join).toHaveBeenCalledTimes(1)
  })
  it('shows retry after initial loading fails', async () => {
    get.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountPage(); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('lottery.loadFailed')
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="lottery-join"]').exists()).toBe(true)
  })
  it('refreshes balance for new winnings and stops polling on unmount', async () => {
    vi.useFakeTimers()
    const wrapper = mountPage(); await flushPromises()
    const winner = fixture(); winner.my_wins = [{ round_id: 130, user_label: '', prize_amount: 5, awarded_at: '2026-09-08T00:00:00Z' }]
    get.mockResolvedValue(winner)
    await wrapper.get('button[aria-label="lottery.refresh"]').trigger('click'); await flushPromises()
    expect(refreshUser).toHaveBeenCalledTimes(1)
    wrapper.unmount(); wrappers.splice(wrappers.indexOf(wrapper), 1)
    const requests = get.mock.calls.length
    await vi.advanceTimersByTimeAsync(30000)
    expect(get).toHaveBeenCalledTimes(requests)
  })
})

describe('Lottery administration', () => {
  it('loads and saves next-round rules while showing the current rules', async () => {
    const wrapper = mountPage(true); await flushPromises()
    expect(wrapper.get<HTMLInputElement>('#prize-amount').element.value).toBe('5')
    await wrapper.get('#prize-amount').setValue('2.50')
    await wrapper.get('#participant-target').setValue('100')
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(configure).toHaveBeenCalledWith({ ...config, prize_amount: 2.5, participant_target: 100, turnstile_secret_key: '' })
    expect(wrapper.text()).toContain('lottery.nextRoundHint')
    expect(wrapper.get('[role="status"]').text()).toContain('lottery.saved')
  })
})
