import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import TeamView from '../TeamView.vue'
import type { TeamSnapshot } from '@/api/team'

const { api, showError, copy, createAPI } = vi.hoisted(() => ({
  api: { get: vi.fn(), keys: vi.fn(), groups: vi.fn(), update: vi.fn(), dissolve: vi.fn(), accept: vi.fn(), acceptToken: vi.fn(), create: vi.fn(), recoverBilling: vi.fn(), models: vi.fn(), availableModels: vi.fn() },
  createAPI: vi.fn(),
  showError: vi.fn(), copy: vi.fn()
}))
vi.mock('@/api/team', () => ({ teamAPI: api, createTeamAPI: createAPI }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: copy }) }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

function state(role: 'owner' | 'member' = 'owner'): TeamSnapshot {
  return {
    team: { id: 1, name: 'Example', owner_id: 1, status: 'active', group_ids: [10, 20], default_limits: { daily: 0, weekly: 0, monthly: 0 }, created_at: '' },
    role, members: [{ user_id: 1, email: 'owner@example.com', role: 'owner', allowed_models: [], limits: { daily: 0, weekly: 0, monthly: 0 }, usage: { daily: 1, weekly: 2, monthly: 3, total: 4 }, resets: { daily: null, weekly: null, monthly: null }, joined_at: '' }],
    invitations: [], pending_invitations: []
  }
}
async function open(adminTeamId?: number) {
  const wrapper = mount(TeamView, { props: { adminTeamId }, global: { stubs: {
    RouterLink: { template: '<a><slot /></a>' },
    AppLayout: { template: '<main><slot /></main>' },
    BaseDialog: { props: ['show'], template: '<section v-if="show" role="dialog"><slot /><slot name="footer" /></section>' }
  } } })
  await flushPromises()
  return wrapper
}
function button(wrapper: Awaited<ReturnType<typeof open>>, label: string) {
  const found = wrapper.findAll('button').find(node => node.text() === label)
  if (!found) throw new Error(`Missing ${label}`)
  return found
}
beforeEach(() => {
  vi.clearAllMocks()
  window.history.replaceState({}, '', '/')
  sessionStorage.clear()
  createAPI.mockReturnValue(api)
  api.get.mockResolvedValue(state())
  api.keys.mockResolvedValue([{ id: 7, user_id: 2, email: 'member@example.com', name: 'Member key', key: 'sk-test-secret', status: 'active', quota_used: 5, last_used_at: null }])
  api.groups.mockResolvedValue([{ id: 10, name: 'Subscription', subscription_type: 'subscription', rate_multiplier: 1 }, { id: 20, name: 'Balance', subscription_type: 'standard', rate_multiplier: 1 }])
  api.availableModels.mockResolvedValue(['gpt-5', 'claude-sonnet-4', 'other-model'])
  api.update.mockResolvedValue({})
})

it('uses administrator-scoped detail APIs and never creates an owner key', async () => {
  const wrapper = await open(7)
  expect(createAPI).toHaveBeenCalledWith('/admin/teams/7')
  await button(wrapper, 'team.keys').trigger('click')
  expect(wrapper.text()).not.toContain('team.createKey')
  await button(wrapper, 'team.reveal').trigger('click')
  expect(wrapper.text()).toContain('sk-test-secret')
  await button(wrapper, 'team.settings').trigger('click')
  expect(wrapper.text()).toContain('team.dissolve')
  wrapper.unmount()
})

it('keeps the compact overview and exposes the guide and member details', async () => {
  const wrapper = await open()
  const member = wrapper.get('details.team-member')
  expect(member.attributes('open')).toBeUndefined()
  expect(member.get('summary').text()).toContain('owner@example.com')
  expect(member.findAll('.team-meters dt')).toHaveLength(3)
  expect(member.text()).toContain('team.editLimits')
  expect(wrapper.get('.team-empty').text()).toBe('team.noInvitations')
  await button(wrapper, 'team.guide').trigger('click')
  expect(wrapper.get('[role="dialog"]').text()).toContain('team.guideSteps.routing')
  await button(wrapper, 'team.closeGuide').trigger('click')
  expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  wrapper.unmount()
})

it('accepts a link token only after user confirmation and removes it from URL', async () => {
  const token = 'a'.repeat(64)
  window.history.replaceState({}, '', `/team#invite=${token}`)
  api.get.mockResolvedValue({ ...state(), team: null, role: null, members: [] })
  api.acceptToken.mockResolvedValue({})
  const wrapper = await open()
  expect(window.location.hash).toBe('')
  expect(api.acceptToken).not.toHaveBeenCalled()
  await button(wrapper, 'team.accept').trigger('click')
  await flushPromises()
  expect(api.acceptToken).toHaveBeenCalledWith(token)
  wrapper.unmount()
})

it('allows owner settlement retry and preserves pending state on failure', async () => {
  api.get.mockResolvedValue({ ...state(), pending_requests: 2, pending_billing: 1 })
  api.recoverBilling.mockRejectedValue(new Error('Settlement still unavailable'))
  const wrapper = await open()
  await button(wrapper, 'team.retryBilling').trigger('click')
  await flushPromises()
  expect(api.recoverBilling).toHaveBeenCalledOnce()
  expect(showError).toHaveBeenCalledWith('Settlement still unavailable')
  expect(wrapper.text()).toContain('team.pendingBilling')
  wrapper.unmount()
})

it('does not expose settlement retry to members', async () => {
  api.get.mockResolvedValue({ ...state('member'), pending_requests: 2, pending_billing: 1 })
  const wrapper = await open()
  expect(wrapper.text()).not.toContain('team.retryBilling')
  wrapper.unmount()
})

describe('team management', () => {
  it('members have own-key controls but no settings, invitations or member administration', async () => {
    api.get.mockResolvedValue(state('member'))
    const wrapper = await open()
    expect(wrapper.text()).not.toContain('team.settings')
    expect(wrapper.text()).not.toContain('team.invite')
    expect(wrapper.text()).not.toContain('team.editLimits')
    expect(api.groups).not.toHaveBeenCalled()
    expect(button(wrapper, 'team.leave').exists()).toBe(true)
    await button(wrapper, 'team.keys').trigger('click')
    expect(wrapper.text()).not.toContain('sk-test-secret')
    await button(wrapper, 'team.reveal').trigger('click')
    expect(wrapper.text()).toContain('sk-test-secret')
    await button(wrapper, 'team.copy').trigger('click')
    expect(copy).toHaveBeenCalledWith('sk-test-secret')
    wrapper.unmount()
  })

  it('sends selected group order and locks settings while saving', async () => {
    let resolve!: (value: unknown) => void
    api.update.mockImplementation(() => new Promise(done => { resolve = done }))
    const wrapper = await open()
    await button(wrapper, 'team.settings').trigger('click')
    await wrapper.get('button[aria-label="team.moveDown"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    expect(api.update).toHaveBeenCalledWith({ name: 'Example', group_ids: [20, 10], default_limits: { daily: 0, weekly: 0, monthly: 0 } })
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    resolve({})
    await flushPromises()
    expect(wrapper.get('fieldset').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('requires exact team name and preserves team after unsuccessful dissolution', async () => {
    api.dissolve.mockRejectedValue(new Error('Wait for pending settlement'))
    const wrapper = await open()
    await button(wrapper, 'team.settings').trigger('click')
    await button(wrapper, 'team.dissolve').trigger('click')
    expect(button(wrapper, 'team.confirm').attributes('disabled')).toBeDefined()
    await wrapper.get('[role="dialog"] input').setValue('example')
    expect(button(wrapper, 'team.confirm').attributes('disabled')).toBeDefined()
    await wrapper.get('[role="dialog"] input').setValue('Example')
    await button(wrapper, 'team.confirm').trigger('click')
    await flushPromises()
    expect(api.dissolve).toHaveBeenCalledWith('Example')
    expect(showError).toHaveBeenCalledWith('Wait for pending settlement')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Example')
    wrapper.unmount()
  })

  it('shows enterprise activation information and recipient invitations without public creation', async () => {
    api.get.mockResolvedValue({ ...state(), team: null, role: null, members: [], pending_invitations: [{ id: 9, team_name: 'Invited team', expires_at: '2099-01-01T00:00:00Z' }] })
    api.accept.mockResolvedValue({})
    const wrapper = await open()
    expect(wrapper.text()).toContain('team.enterpriseActivation')
    expect(wrapper.text()).not.toContain('team.create')
    expect(api.keys).not.toHaveBeenCalled()
    await button(wrapper, 'team.accept').trigger('click')
    await flushPromises()
    expect(api.accept).toHaveBeenCalledWith(9)
    wrapper.unmount()
  })

  it('clears team and full keys after successful dissolution', async () => {
    api.dissolve.mockResolvedValue({})
    const wrapper = await open()
    await button(wrapper, 'team.keys').trigger('click')
    await button(wrapper, 'team.reveal').trigger('click')
    expect(wrapper.text()).toContain('sk-test-secret')
    await button(wrapper, 'team.settings').trigger('click')
    await button(wrapper, 'team.dissolve').trigger('click')
    await wrapper.get('[role="dialog"] input').setValue('Example')
    api.get.mockResolvedValue({ team: null, role: null, members: [], invitations: [], pending_invitations: [] })
    await button(wrapper, 'team.confirm').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('team.enterpriseActivation')
    expect(wrapper.text()).not.toContain('team.create')
    expect(wrapper.text()).not.toContain('sk-test-secret')
    expect(wrapper.text()).not.toContain('Example')
    wrapper.unmount()
  })
})


it('requires the administrator link and keeps it available when activation fails', async () => {
  const token = 'b'.repeat(64)
  window.history.replaceState({}, '', `/team#activate=${token}`)
  api.get.mockResolvedValue({ ...state(), team: null, role: null, members: [] })
  api.create.mockRejectedValueOnce({reason: 'TEAM_ACTIVATION_REQUIRED', message: 'Invitation expired'})
  let wrapper = await open()
  expect(window.location.hash).toBe('')
  expect(api.create).not.toHaveBeenCalled()
  expect(wrapper.text()).toContain('team.activate')
  await wrapper.get('form input').setValue('Enterprise')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(api.create).toHaveBeenCalledWith('Enterprise', token)
  expect(showError).toHaveBeenCalledWith('team.activationInvalid')
  wrapper.unmount()
  wrapper = await open()
  expect(wrapper.text()).toContain('team.activate')
  await wrapper.get('form input').setValue('Enterprise')
  api.create.mockResolvedValueOnce({})
  api.get.mockResolvedValue(state())
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(sessionStorage.getItem('pending_team_activation')).toBeNull()
  expect(wrapper.text()).toContain('team.sharedQuotaHint')
  wrapper.unmount()
})

for (const adminTeamId of [undefined, 7]) {
  it(`saves and clears member model restrictions (${adminTeamId ? 'admin' : 'owner'})`, async () => {
    api.models.mockResolvedValue({})
    const wrapper = await open(adminTeamId)
    await button(wrapper, 'team.editModels').trigger('click')
    await flushPromises()
    expect(api.availableModels).toHaveBeenCalledOnce()
    const dialog = wrapper.get('[role="dialog"]')
    await dialog.get('input[value="gpt-5"]').setValue(true)
    await dialog.get('input[value="claude-sonnet-4"]').setValue(true)
    await dialog.get('form').trigger('submit')
    await flushPromises()
    expect(api.models).toHaveBeenLastCalledWith(1, ['gpt-5', 'claude-sonnet-4'])
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    await button(wrapper, 'team.editModels').trigger('click')
    await flushPromises()
    await button(wrapper, 'team.allModels').trigger('click')
    await wrapper.get('[role="dialog"] form').trigger('submit')
    await flushPromises()
    expect(api.models).toHaveBeenLastCalledWith(1, [])
    wrapper.unmount()
  })
}

it('shows member model restrictions without editing controls', async () => {
  const restricted = state('member')
  restricted.members[0].allowed_models = ['gpt-5']
  api.get.mockResolvedValue(restricted)
  const wrapper = await open()
  expect(wrapper.text()).toContain('gpt-5')
  expect(wrapper.text()).not.toContain('team.editModels')
  expect(api.availableModels).not.toHaveBeenCalled()
  expect(api.models).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('preserves existing restrictions when a model becomes unavailable and supports search', async () => {
  const restricted = state()
  restricted.members[0].allowed_models = ['old-model', 'gpt-5']
  api.get.mockResolvedValue(restricted)
  const wrapper = await open()
  await button(wrapper, 'team.editModels').trigger('click')
  await flushPromises()
  const dialog = wrapper.get('[role="dialog"]')
  expect((dialog.get('input[value="old-model"]').element as HTMLInputElement).checked).toBe(true)
  expect(dialog.text()).toContain('team.modelUnavailable')
  await dialog.get('input[aria-label="team.searchModels"]').setValue('gpt')
  expect(dialog.findAll('input[type="checkbox"]')).toHaveLength(1)
  api.models.mockResolvedValue({})
  await dialog.get('form').trigger('submit')
  await flushPromises()
  expect(api.models).toHaveBeenCalledWith(1, ['old-model', 'gpt-5'])
  wrapper.unmount()
})

it('blocks saving on catalog failure and retries without clearing restrictions', async () => {
  api.availableModels.mockRejectedValueOnce(new Error('Unavailable'))
  const restricted = state()
  restricted.members[0].allowed_models = ['gpt-5']
  api.get.mockResolvedValue(restricted)
  const wrapper = await open()
  await button(wrapper, 'team.editModels').trigger('click')
  await flushPromises()
  const dialog = wrapper.get('[role="dialog"]')
  expect(dialog.text()).toContain('team.modelsLoadFailed')
  expect(dialog.findAll('button').find(node => node.text() === 'team.save')!.attributes('disabled')).toBeDefined()
  await dialog.get('form').trigger('submit')
  expect(api.models).not.toHaveBeenCalled()
  await dialog.findAll('button').find(node => node.text() === 'team.refresh')!.trigger('click')
  await flushPromises()
  expect((dialog.get('input[value="gpt-5"]').element as HTMLInputElement).checked).toBe(true)
  wrapper.unmount()
})

it('explains the model preview when the team has no routing pools', async () => {
  const unconfigured = state()
  unconfigured.team!.group_ids = []
  api.get.mockResolvedValue(unconfigured)
  const wrapper = await open()
  await button(wrapper, 'team.editModels').trigger('click')
  await flushPromises()
  expect(wrapper.get('[role="dialog"]').text()).toContain('team.modelsRoutingHint')
  expect(wrapper.get('[role="dialog"] input[value="gpt-5"]').exists()).toBe(true)
  expect(api.update).not.toHaveBeenCalled()
  wrapper.unmount()
})
