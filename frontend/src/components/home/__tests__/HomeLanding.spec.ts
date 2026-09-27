import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import HomeLanding from '../HomeLanding.vue'
import { getModelPlaza, type ModelPlazaGroup, type ModelPlazaResponse } from '@/api/modelPlaza'

vi.mock('@/api/modelPlaza', () => ({ getModelPlaza: vi.fn() }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key} ${JSON.stringify(params)}` : key,
  }),
}))

const wrappers: ReturnType<typeof mount>[] = []
function mountLanding(props: Record<string, unknown> = {}) {
  const wrapper = mount(HomeLanding, {
    props: {
      siteName: 'Test site',
      siteLogo: '',
      siteSubtitle: 'Test subtitle',
      docUrl: '',
      isAuthenticated: false,
      dashboardPath: '/dashboard',
      isDark: false,
      showModelPlaza: true,
      registrationEnabled: false,
      ...props,
    },
    global: {
      stubs: { RouterLink: RouterLinkStub, LocaleSwitcher: true, ModelIcon: true, Icon: true },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

function group(id: number, names: string[]): ModelPlazaGroup {
  return {
    id,
    platform: 'openai',
    models: names.map((name) => ({
      name,
      platform: 'openai',
      pricing: null,
      official_pricing: null,
    })),
  } as ModelPlazaGroup
}

describe('HomeLanding', () => {
  beforeEach(() => {
    vi.mocked(getModelPlaza).mockReset().mockResolvedValue({ groups: [], description: '' })
  })
  afterEach(() => {
    wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it('does not request or link to a catalog that visitors cannot access', async () => {
    const wrapper = mountLanding({ showModelPlaza: false })
    await flushPromises()
    expect(getModelPlaza).not.toHaveBeenCalled()
    expect(wrapper.find('#models').exists()).toBe(false)
    expect(
      wrapper.findAllComponents(RouterLinkStub).some((link) => link.props('to') === '/model-plaza'),
    ).toBe(false)
    expect(wrapper.get('.hero-actions a[href="#quickstart"]').exists()).toBe(true)
  })

  it('counts unique models across service groups using catalog data', async () => {
    vi.mocked(getModelPlaza).mockResolvedValue({
      groups: [group(1, ['gpt-a', 'gpt-b']), group(2, ['gpt-a'])],
      description: '',
    })
    const wrapper = mountLanding()
    expect(wrapper.get('#models').attributes('aria-busy')).toBe('true')
    await flushPromises()
    expect(wrapper.findAll('.model-card')).toHaveLength(1)
    expect(wrapper.get('.model-card-details strong').text()).toBe('2')
    expect(wrapper.get('.model-card-heading p').text()).toContain('"count":2')
    expect(wrapper.get('#models').text()).toContain('"models":2,"platforms":1')
  })

  it('offers a working retry after a failed catalog request', async () => {
    vi.mocked(getModelPlaza).mockRejectedValueOnce(new Error('Unavailable'))
    const wrapper = mountLanding()
    await flushPromises()
    expect(wrapper.get('.catalog-notice').text()).toContain('home.landing.modelsUnavailable')
    vi.mocked(getModelPlaza).mockResolvedValueOnce({
      groups: [group(1, ['gpt-a'])],
      description: '',
    })
    await wrapper.get('.catalog-notice button').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.model-card')).toHaveLength(1)
  })

  it('discards in-flight data when catalog access is revoked', async () => {
    let finish!: (response: ModelPlazaResponse) => void
    vi.mocked(getModelPlaza).mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve
      }),
    )
    const wrapper = mountLanding()
    const signal = vi.mocked(getModelPlaza).mock.calls[0][0]?.signal
    await wrapper.setProps({ showModelPlaza: false })
    expect(signal?.aborted).toBe(true)
    finish({ groups: [group(1, ['gpt-a'])], description: '' })
    await flushPromises()
    expect(wrapper.find('#models').exists()).toBe(false)
  })

  it('respects registration settings and the authenticated dashboard destination', async () => {
    const wrapper = mountLanding()
    const cta = () => wrapper.get('.closing-cta').findComponent(RouterLinkStub).props('to')
    expect(cta()).toBe('/login')
    await wrapper.setProps({ registrationEnabled: true })
    expect(cta()).toBe('/register')
    await wrapper.setProps({ isAuthenticated: true, dashboardPath: '/admin/dashboard' })
    expect(cta()).toBe('/admin/dashboard')
    expect(wrapper.get('.hero-actions').findComponent(RouterLinkStub).props('to')).toBe(
      '/admin/dashboard',
    )
  })
})
