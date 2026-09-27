import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { lotteryAPI, type LotterySnapshot } from '@/api/lottery'
import { useLotteryStore } from '../lottery'

const { auth } = vi.hoisted(() => ({ auth: { value: {} as { isAuthenticated: boolean; isSimpleMode: boolean; user: { id: number } | null } } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth.value }))
vi.mock('@/api/lottery', () => ({ lotteryAPI: { get: vi.fn() } }))
const snapshot = (enabled: boolean) => ({ config: { enabled } }) as LotterySnapshot

describe('lottery navigation availability', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    auth.value = reactive({ isAuthenticated: true, isSimpleMode: false, user: { id: 1 } })
    vi.mocked(lotteryAPI.get).mockReset()
  })
  it.each([false, true])('shows the entry only when configured enabled=%s', async enabled => {
    vi.mocked(lotteryAPI.get).mockResolvedValue(snapshot(enabled))
    const store = useLotteryStore()
    expect(store.enabled).toBe(false)
    await store.refresh()
    expect(store.enabled).toBe(enabled)
  })
  it('hides the entry on request failure', async () => {
    vi.mocked(lotteryAPI.get).mockRejectedValue(new Error('unavailable'))
    const store = useLotteryStore()
    await store.refresh()
    expect(store.enabled).toBe(false)
  })
  it.each(['anonymous', 'simple'])('skips activity requests in %s mode', async mode => {
    auth.value.isAuthenticated = mode !== 'anonymous'
    auth.value.isSimpleMode = mode === 'simple'
    const store = useLotteryStore()
    await store.refresh()
    expect(lotteryAPI.get).not.toHaveBeenCalled()
    expect(store.enabled).toBe(false)
  })
  it('deduplicates requests and caches the result across sidebar mounts', async () => {
    vi.mocked(lotteryAPI.get).mockResolvedValue(snapshot(true))
    const store = useLotteryStore()
    await Promise.all([store.refresh(), store.refresh()])
    await store.refresh()
    expect(lotteryAPI.get).toHaveBeenCalledOnce()
  })
  it('does not overwrite an admin toggle with an older request', async () => {
    let finish!: (value: LotterySnapshot) => void
    vi.mocked(lotteryAPI.get).mockReturnValue(new Promise(resolve => { finish = resolve }))
    const store = useLotteryStore()
    const request = store.refresh()
    store.setEnabled(false)
    finish(snapshot(true))
    await request
    expect(store.enabled).toBe(false)
    store.setEnabled(true)
    expect(store.enabled).toBe(true)
  })
  it('does not carry an old session response into another account', async () => {
    let finish!: (value: LotterySnapshot) => void
    vi.mocked(lotteryAPI.get).mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const store = useLotteryStore()
    const request = store.refresh()
    auth.value.user = { id: 2 }
    finish(snapshot(true))
    await request
    expect(store.enabled).toBe(false)
    vi.mocked(lotteryAPI.get).mockResolvedValueOnce(snapshot(true))
    await store.refresh()
    expect(store.enabled).toBe(true)
    auth.value.isAuthenticated = false
    expect(store.enabled).toBe(false)
  })
})
