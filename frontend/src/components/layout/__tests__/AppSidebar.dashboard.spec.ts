import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import AppSidebar from '../AppSidebar.vue'

const mocks = vi.hoisted(() => ({
  auth: {} as { isAdmin: boolean; isSimpleMode: boolean },
  app: {} as Record<string, any>,
  admin: {} as Record<string, any>,
  route: { path: '/dashboard' }
}))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', async () => ({ ...await vi.importActual<typeof import('vue-router')>('vue-router'), useRoute: () => mocks.route, useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/stores', () => ({
  useAppStore: () => mocks.app, useAuthStore: () => mocks.auth,
  useAdminSettingsStore: () => mocks.admin,
  useOnboardingStore: () => ({ isCurrentStep: () => false })
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks.app }))
vi.mock('@/composables/useBatchImageAccess', () => ({ useBatchImageAccess: () => ({ canUseBatchImage: ref(false), refreshBatchImageAccess: vi.fn() }) }))

const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const wrapper = mount(AppSidebar, { global: { stubs: {
    VersionBadge: true,
    RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' }
  } } })
  wrappers.push(wrapper)
  return wrapper
}

describe('consolidated dashboard navigation', () => {
  beforeEach(() => {
    mocks.auth = reactive({ isAdmin: false, isSimpleMode: false })
    mocks.app = reactive({
      sidebarCollapsed: false, mobileOpen: false, sidebarScrollTop: 0, backendModeEnabled: false,
      publicSettingsLoaded: true, siteName: 'Test', siteLogo: '', siteVersion: 'test',
      cachedPublicSettings: { payment_enabled: true, available_channels_enabled: true, channel_monitor_enabled: true, custom_menu_items: [{ id: 'help', label: 'Help', visibility: 'user', sort_order: 0, icon_svg: '', open_mode: 'new_tab' }] }
    })
    mocks.admin = reactive({ fetch: vi.fn(), customMenuItems: [], opsMonitoringEnabled: false, paymentEnabled: true })
  })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

  it('removes only subscription and redeem self entries while preserving profile and commerce', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('nav a[href="/subscriptions"]').exists()).toBe(false)
    expect(wrapper.find('nav a[href="/redeem"]').exists()).toBe(false)
    for (const path of ['/dashboard', '/keys', '/usage', '/available-channels', '/monitor', '/purchase', '/package-groups', '/orders', '/lottery', '/profile', '/custom/help']) {
      expect(wrapper.find(`nav a[href="${path}"]`).exists(), path).toBe(true)
    }
    expect(wrapper.get('nav a[href="/profile"]').text()).toBe('nav.profile')
    expect(wrapper.get('nav a[href="/custom/help"]').attributes('target')).toBe('_blank')
  })

  it('keeps admin management and provides its own account overview entry', async () => {
    mocks.auth.isAdmin = true
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('nav a[href="/admin/dashboard"]').text()).toBe('nav.dashboard')
    expect(wrapper.get('nav a[href="/dashboard"]').text()).toBe('dashboard.accountOverview')
    expect(wrapper.find('nav a[href="/admin/subscriptions"]').exists()).toBe(true)
    expect(wrapper.find('nav a[href="/admin/redeem"]').exists()).toBe(true)
    expect(wrapper.find('nav a[href="/profile"]').exists()).toBe(true)
    expect(wrapper.find('nav a[href="/subscriptions"]').exists()).toBe(false)
    expect(wrapper.find('nav a[href="/redeem"]').exists()).toBe(false)
  })

  it('preserves feature flags and simple-mode visibility', async () => {
    mocks.app.cachedPublicSettings.payment_enabled = false
    mocks.app.cachedPublicSettings.available_channels_enabled = false
    mocks.app.cachedPublicSettings.channel_monitor_enabled = false
    const wrapper = render()
    await flushPromises()
    for (const path of ['/purchase', '/package-groups', '/orders', '/available-channels', '/monitor']) expect(wrapper.find(`nav a[href="${path}"]`).exists()).toBe(false)
    mocks.auth.isSimpleMode = true
    await flushPromises()
    for (const path of ['/usage', '/lottery']) expect(wrapper.find(`nav a[href="${path}"]`).exists()).toBe(false)
    for (const path of ['/dashboard', '/keys', '/profile']) expect(wrapper.find(`nav a[href="${path}"]`).exists()).toBe(true)
  })
})
