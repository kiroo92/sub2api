import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LotteryView from '../LotteryView.vue'
import AdminLotteryView from '@/views/admin/LotteryView.vue'
import type { LotterySnapshot } from '@/api/lottery'

const { get, join, adminGet, configure, refreshUser, verifyAction, resetCaptcha, getPublicSettings } = vi.hoisted(() => ({ get: vi.fn(), join: vi.fn(), adminGet: vi.fn(), configure: vi.fn(), refreshUser: vi.fn(), verifyAction: vi.fn(), resetCaptcha: vi.fn(), getPublicSettings: vi.fn() }))
vi.mock('@/api/auth', () => ({ getPublicSettings }))
vi.mock('@/components/CaptchaChallenge.vue', () => ({ default: defineComponent({ setup(_, { expose }) { expose({ verifyAction, reset: resetCaptcha }); return () => null } }) }))
vi.mock('@/api/lottery', () => ({ lotteryAPI: { get, join, adminGet, configure } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }) }))
const config = { enabled: true, prize_amount: 5, winner_count: 6, participant_target: 60, min_recharge: 50 }
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
beforeEach(() => { vi.clearAllMocks(); verifyAction.mockResolvedValue({ token: 'ticket', randstr: 'rand' }); getPublicSettings.mockResolvedValue({ tencent_captcha_enabled: true, tencent_captcha_app_id: '123' }); get.mockResolvedValue(fixture()); adminGet.mockResolvedValue(fixture()); configure.mockResolvedValue(config); refreshUser.mockResolvedValue({}) })
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })

describe('Lottery participation', () => {
  it('never submits if the challenge is cancelled', async () => {
    verifyAction.mockResolvedValue(null)
    const wrapper = mountPage(); await flushPromises()
    await wrapper.get('[data-testid="lottery-join"]').trigger('click'); await flushPromises()
    expect(verifyAction).toHaveBeenCalledOnce(); expect(join).not.toHaveBeenCalled()
    expect(resetCaptcha).toHaveBeenCalledTimes(2)
  })
  it('blocks entry without configured slider verification', async () => {
    getPublicSettings.mockResolvedValue({})
    const wrapper = mountPage(); await flushPromises()
    expect(wrapper.get('[data-testid="lottery-join"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('lottery.captchaUnavailable')
  })
  it('waits for verification before sending an Aliyun proof', async () => {
    getPublicSettings.mockResolvedValue({ aliyun_captcha_enabled: true, aliyun_captcha_scene_id: 'scene', aliyun_captcha_prefix: 'prefix' })
    let verified!: (value: unknown) => void
    verifyAction.mockReturnValue(new Promise(resolve => { verified = resolve }))
    join.mockResolvedValue({ drawn: false })
    const wrapper = mountPage(); await flushPromises()
    await wrapper.get('[data-testid="lottery-join"]').trigger('click')
    expect(join).not.toHaveBeenCalled()
    verified({ token: 'aliyun-proof', randstr: '' }); await flushPromises()
    expect(join).toHaveBeenCalledWith(131, { turnstile_token: 'aliyun-proof' })
  })
  it('loads progress and submits only the displayed round once while pending', async () => {
    let resolve!: (value: unknown) => void
    join.mockReturnValue(new Promise(done => { resolve = done }))
    const wrapper = mountPage(); await flushPromises()
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('11')
    const button = wrapper.get('[data-testid="lottery-join"]')
    await button.trigger('click'); await button.trigger('click')
    expect(join).toHaveBeenCalledTimes(1); expect(join).toHaveBeenCalledWith(131, { tencent_captcha_ticket: 'ticket', tencent_captcha_randstr: 'rand' })
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
    await wrapper.get('[data-testid="lottery-join"]').trigger('click'); await flushPromises()
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
    expect(configure).toHaveBeenCalledWith({ ...config, prize_amount: 2.5, participant_target: 100 })
    expect(wrapper.text()).toContain('lottery.nextRoundHint')
    expect(wrapper.get('[role="status"]').text()).toContain('lottery.saved')
  })
})
