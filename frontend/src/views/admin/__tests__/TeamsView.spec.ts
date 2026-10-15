import { beforeEach, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import TeamsView from '../TeamsView.vue'

const { api, error } = vi.hoisted(() => ({ api: { list: vi.fn(), config: vi.fn(), saveConfig: vi.fn(), activationInvitations: vi.fn(), inviteActivation: vi.fn(), revokeActivation: vi.fn() }, error: vi.fn() }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
vi.mock('@/api/team', () => ({ adminTeamsAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: error, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
function open() {
  return mount(TeamsView, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
    EmailTemplateEditor: { props: ['eventFilter'], template: '<div data-testid="template">{{ eventFilter }}</div>' },
    Icon: true
  } } })
}
beforeEach(() => {
  vi.clearAllMocks()
  api.list.mockResolvedValue({ items: [{ id: 7, name: 'Team seven', owner_email: 'owner@example.com', status: 'active', member_count: 3, total_usage: 4 }], total: 21 })
  api.config.mockResolvedValue({ frontend_url: 'https://example.com' })
  api.saveConfig.mockResolvedValue({})
  api.activationInvitations.mockResolvedValue([{ id: 5, email: 'owner@example.com', expires_at: '2099-01-01T00:00:00Z' }])
  api.inviteActivation.mockResolvedValue({ invitation_url: `https://example.com/team#activate=${'a'.repeat(64)}` })
  api.revokeActivation.mockResolvedValue({})
})
it('lists teams, filters and links to ID-scoped administration', async () => {
  const wrapper = open(); await flushPromises()
  expect(wrapper.get('a').attributes('href')).toBe('/admin/teams/7')
  await wrapper.get('input').setValue('owner@example.com')
  await wrapper.get('select').setValue('paused')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.list).toHaveBeenLastCalledWith({ search: 'owner@example.com', status: 'paused', page: 1, page_size: 20 })
  await wrapper.findAll('button').find(b => b.text() === 'team.next')!.trigger('click'); await flushPromises()
  expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
  wrapper.unmount()
})
it('uses shared URL configuration and scopes template editor to team invitations', async () => {
  const wrapper = open(); await flushPromises()
  await wrapper.findAll('button').find(b => b.text() === 'team.invitationConfig')!.trigger('click'); await flushPromises()
  expect(wrapper.get('[data-testid="template"]').text()).toBe('team.invitation')
  expect((wrapper.get('input').element as HTMLInputElement).value).toBe('https://example.com')
  await wrapper.get('input').setValue('https://new.example.com')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.saveConfig).toHaveBeenCalledWith('https://new.example.com')
  wrapper.unmount()
})

it('sends, resends and revokes enterprise activation invitations', async () => {
  const wrapper = open(); await flushPromises()
  await wrapper.findAll('button').find(b => b.text() === 'team.activationInvitations')!.trigger('click'); await flushPromises()
  await wrapper.get('input[type="email"]').setValue('new@example.com')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(api.inviteActivation).toHaveBeenCalledWith('new@example.com')
  expect((wrapper.get('input[readonly]').element as HTMLInputElement).value).toContain('#activate=')
  await wrapper.findAll('button').find(b => b.text() === 'team.resend')!.trigger('click'); await flushPromises()
  expect(api.inviteActivation).toHaveBeenLastCalledWith('owner@example.com')
  await wrapper.findAll('button').find(b => b.text() === 'team.revoke')!.trigger('click'); await flushPromises()
  expect(api.revokeActivation).toHaveBeenCalledWith(5)
  expect(wrapper.find('input[readonly]').exists()).toBe(false)
  wrapper.unmount()
})
