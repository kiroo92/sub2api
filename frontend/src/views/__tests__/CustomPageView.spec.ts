import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CustomPageView from '../user/CustomPageView.vue'

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  item: { id: 'image', label: 'Image', url: 'https://image.example/image/', open_mode: undefined as string | undefined },
  admin: false,
  visible: true,
}))
vi.mock('@/stores', () => ({ useAppStore: () => ({
  publicSettingsLoaded: true,
  cachedPublicSettings: { custom_menu_items: mocks.visible ? [mocks.item] : [] },
}) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: mocks.admin, isAuthenticated: true, user: { id: 42 }, token: 'fixture-token' }) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: mocks.admin ? [mocks.item] : [] }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: 'image' } }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh-CN' } }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => path }))

describe('custom menu open mode', () => {
  afterEach(() => vi.unstubAllGlobals())
  beforeEach(() => {
    mocks.replace.mockReset()
    mocks.item.open_mode = undefined
    mocks.admin = false
    mocks.visible = true
    vi.stubGlobal('location', { origin: 'https://sub.example', href: 'https://sub.example/custom/image', replace: mocks.replace })
  })

  it('keeps existing menus embedded by default', () => {
    const wrapper = mount(CustomPageView)
    expect(wrapper.find('iframe').exists()).toBe(true)
    expect(mocks.replace).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('opens a standalone page with website credentials and renders no iframe', () => {
    mocks.item.open_mode = 'new_tab'
    const wrapper = mount(CustomPageView)
    const url = new URL(mocks.replace.mock.calls[0][0])
    expect(url.searchParams.get('ui_mode')).toBe('standalone')
    expect(url.searchParams.get('token')).toBe('fixture-token')
    expect(url.searchParams.get('src_host')).toBe('https://sub.example')
    expect(wrapper.find('iframe').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not open an admin menu for a regular user', () => {
    mocks.visible = false
    mocks.item.open_mode = 'new_tab'
    const wrapper = mount(CustomPageView)
    expect(mocks.replace).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('customPage.notFoundTitle')
    wrapper.unmount()
  })
})
