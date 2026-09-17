<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { teamAPI as userTeamAPI, createTeamAPI, type TeamKey, type TeamLimits, type TeamMember, type TeamSnapshot } from '@/api/team'
import type { Group } from '@/types'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { rememberTeamInvitation, takeTeamInvitation } from '@/utils/teamInvitation'

const { t } = useI18n()
const props = defineProps<{ adminTeamId?: number }>()
const teamAPI = props.adminTeamId ? createTeamAPI(`/admin/teams/${props.adminTeamId}`) : userTeamAPI
const invitationToken = ref('')
const app = useAppStore()
const { copyToClipboard } = useClipboard()
const snapshot = ref<TeamSnapshot | null>(null)
const groups = ref<Group[]>([])
const keys = ref<TeamKey[]>([])
const busy = ref(false)
const loading = ref(true)
const error = ref('')
const tab = ref<'overview' | 'keys' | 'settings'>('overview')
const windows = ['daily', 'weekly', 'monthly'] as const
const team = computed(() => snapshot.value?.team)
const owner = computed(() => snapshot.value?.role === 'owner')
const name = ref('')
const email = ref('')
const keyName = ref('')
const selected = ref<number[]>([])
const defaults = ref<TeamLimits>({ daily: 0, weekly: 0, monthly: 0 })
const revealed = ref<number[]>([])
const editingKey = ref<TeamKey | null>(null)
const editingMember = ref<TeamMember | null>(null)
const memberLimits = ref<TeamLimits>({ daily: 0, weekly: 0, monthly: 0 })
const confirmation = ref<{ title: string; message: string; run: () => Promise<unknown>; dissolve?: boolean } | null>(null)
const confirmName = ref('')
const showGuide = ref(false)

function report(reason: unknown) {
  error.value = reason && typeof reason === 'object' && 'message' in reason ? String(reason.message) : t('team.failed')
  app.showError(error.value)
}
async function load() {
  const state = await teamAPI.get()
  snapshot.value = state
  // Clear full keys before another membership or a dissolved team can be rendered.
  keys.value = []
  revealed.value = []
  groups.value = []
  if (state.team) {
    name.value = state.team.name
    selected.value = [...state.team.group_ids]
    defaults.value = { ...state.team.default_limits }
    keys.value = await teamAPI.keys()
    if (state.role === 'owner') groups.value = await teamAPI.groups()
  } else {
    name.value = ''
    tab.value = 'overview'
  }
}
async function refresh() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try { await load() } catch (reason) { report(reason) }
  finally { busy.value = false; loading.value = false }
}
async function mutate(action: () => Promise<unknown>, done?: () => void) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await action()
    done?.()
    await load()
    app.showSuccess(t('team.saved'))
  } catch (reason) { report(reason) }
  finally { busy.value = false }
}
function move(index: number, direction: number) {
  if (busy.value || index + direction < 0 || index + direction >= selected.value.length) return
  const next = [...selected.value]
  const [id] = next.splice(index, 1)
  next.splice(index + direction, 0, id)
  selected.value = next
}
function editLimits(member: TeamMember) {
  editingMember.value = member
  memberLimits.value = { ...member.limits }
}
function ask(title: string, message: string, run: () => Promise<unknown>, dissolve = false) {
  confirmName.value = ''
  confirmation.value = { title, message, run, dissolve }
}
function confirm() {
  const action = confirmation.value
  if (!action || (action.dissolve && confirmName.value !== team.value?.name)) return
  void mutate(action.run, () => { confirmation.value = null })
}
function saveKey() {
  const key = editingKey.value
  if (!key || !keyName.value.trim()) return
  void mutate(() => teamAPI.updateKey(key.id, { name: keyName.value.trim() }), () => { editingKey.value = null; keyName.value = '' })
}
onMounted(() => {
  if (!props.adminTeamId) {
    rememberTeamInvitation(window.location.hash)
    invitationToken.value = takeTeamInvitation()
    if (invitationToken.value) window.history.replaceState(window.history.state, '', window.location.pathname + window.location.search)
  }
  void refresh()
})
</script>

<template>
  <AppLayout class="team-layout">
    <div class="team-page">
      <RouterLink v-if="adminTeamId" to="/admin/teams" class="text-sm text-primary-600">{{ t('team.backToTeams') }}</RouterLink>
      <header class="team-heading">
        <div><h1>{{ t('team.title') }}</h1><p>{{ t('team.description') }}</p></div>
        <button v-if="!team" class="btn btn-secondary" :disabled="busy" @click="refresh"><Icon name="refresh" size="sm" aria-hidden="true" />{{ t('team.refresh') }}</button>
      </header>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-red-700 dark:bg-red-950 dark:text-red-300">{{ error }}</p>
      <p v-if="loading" role="status" class="team-muted">{{ t('team.loading') }}</p>
      <template v-else-if="snapshot">
        <p v-if="invitationToken && team" role="status" class="text-amber-700">{{ t('team.alreadyInTeam') }}</p>
        <template v-if="!team && !adminTeamId">
          <section v-if="invitationToken" class="space-y-3 border-b pb-5 dark:border-dark-700">
            <h2 class="font-semibold">{{ t('team.linkInvitation') }}</h2>
            <p class="text-sm text-gray-500">{{ t('team.invitationEmailHint') }}</p>
            <button class="btn btn-primary" :disabled="busy" @click="mutate(() => teamAPI.acceptToken(invitationToken), () => { invitationToken = '' })">{{ t('team.accept') }}</button>
          </section>
          <form class="card max-w-xl space-y-4 p-6" @submit.prevent="mutate(() => teamAPI.create(name.trim()))">
            <h2 class="text-lg font-semibold">{{ t('team.create') }}</h2>
            <p class="text-sm text-gray-500">{{ t('team.createHint') }}</p>
            <label class="block">{{ t('team.name') }}<input v-model="name" class="input mt-2" required maxlength="100" :disabled="busy" /></label>
            <button class="btn btn-primary" :disabled="busy || !name.trim()">{{ t('team.create') }}</button>
          </form>
          <section v-if="snapshot.pending_invitations.length" class="card space-y-4 p-6">
            <h2 class="font-semibold">{{ t('team.pendingInvitations') }}</h2>
            <div v-for="invite in snapshot.pending_invitations" :key="invite.id" class="flex flex-wrap items-center justify-between gap-3 border-t pt-4 dark:border-dark-700">
              <div><p>{{ invite.team_name }}</p><p class="text-sm text-gray-500">{{ t('team.expires') }}: {{ formatDateTime(invite.expires_at) }}</p></div>
              <button class="btn btn-primary" :disabled="busy" @click="mutate(() => teamAPI.accept(invite.id))">{{ t('team.accept') }}</button>
            </div>
          </section>
        </template>
        <template v-else-if="team">
          <div class="team-summary">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2.5"><h2 class="team-name">{{ team.name }}</h2><span class="team-badge" :class="team.status === 'active' ? 'team-badge-active' : 'team-badge-paused'">{{ t(`team.${team.status}`) }}</span></div>
              <p class="team-muted mt-1.5">{{ t(adminTeamId ? 'team.platformAdmin' : owner ? 'team.owner' : 'team.member') }}<template v-if="owner"> · {{ t('team.memberCount', { count: snapshot.members.length }) }}</template></p>
            </div>
            <div class="team-toolbar">
              <button class="btn btn-secondary" @click="showGuide = true"><Icon name="questionCircle" size="sm" aria-hidden="true" />{{ t('team.guide') }}</button>
              <button class="btn btn-secondary" :disabled="busy" @click="refresh"><Icon name="refresh" size="sm" :class="{ 'animate-spin': busy }" aria-hidden="true" />{{ t('team.refresh') }}</button>
            </div>
          </div>
          <p v-if="team.status !== 'active'" role="status" class="rounded-lg bg-amber-50 p-3 text-amber-800 dark:bg-amber-950 dark:text-amber-200">{{ t('team.pausedHint') }}</p>
          <nav class="team-tabs" :aria-label="t('team.title')">
            <button v-for="item in (owner ? ['overview', 'keys', 'settings'] as const : ['overview', 'keys'] as const)" :key="item" :class="{ 'is-active': tab === item }" :aria-current="tab === item ? 'page' : undefined" @click="tab = item">{{ t(`team.${item}`) }}</button>
          </nav>
          <section v-if="tab === 'overview'" class="team-overview">
            <div v-if="owner && snapshot.pending_requests" class="flex flex-wrap items-center justify-between gap-3 border-l-4 border-amber-500 bg-amber-50 p-4 dark:bg-amber-950" role="status">
              <p>{{ t('team.pendingBilling', { requests: snapshot.pending_requests, bills: snapshot.pending_billing || 0 }) }}</p>
              <button v-if="snapshot.pending_billing" class="btn btn-secondary" :disabled="busy" @click="mutate(teamAPI.recoverBilling)">{{ t('team.retryBilling') }}</button>
            </div>
            <section class="team-panel" :aria-label="t('team.members')">
              <div class="team-panel-heading"><h3>{{ t('team.members') }}</h3><span v-if="owner" class="team-muted">{{ t('team.memberCount', { count: snapshot.members.length }) }}</span></div>
              <details v-for="member in snapshot.members" :key="member.user_id" class="team-member">
                <summary class="team-member-summary">
                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2"><span class="team-member-name">{{ member.username || member.email }}</span><span class="team-badge team-badge-role">{{ t(`team.${member.role}`) }}</span></div>
                    <p class="team-muted mt-1 break-all">{{ member.email }}</p>
                  </div>
                  <span class="team-detail-toggle">{{ t('team.usageDetails') }}<Icon name="chevronDown" size="xs" class="team-chevron" aria-hidden="true" /></span>
                </summary>
                <div class="team-member-details">
                  <dl class="team-meters">
                    <div v-for="period in windows" :key="period"><dt>{{ t(`team.${period}`) }}</dt><dd>{{ formatCurrency(member.usage[period]) }} <span class="team-muted">/ {{ member.limits[period] === 0 ? t('team.unlimited') : formatCurrency(member.limits[period]) }}</span><p class="team-muted mt-1 text-xs">{{ t('team.resets') }}: {{ formatDateTime(member.resets[period]) }}</p></dd></div>
                  </dl>
                  <div class="mt-5 flex flex-wrap items-center justify-between gap-3">
                    <p class="team-muted">{{ t('team.total') }}: {{ formatCurrency(member.usage.total) }}</p>
                    <div v-if="owner" class="flex flex-wrap gap-2"><button class="btn btn-secondary btn-sm" :disabled="busy" @click="editLimits(member)">{{ t('team.editLimits') }}</button><button v-if="member.role !== 'owner'" class="btn btn-danger btn-sm" :disabled="busy" @click="ask(t('team.remove'), t('team.removeHint'), () => teamAPI.remove(member.user_id))">{{ t('team.remove') }}</button></div>
                  </div>
                </div>
              </details>
            </section>
            <section v-if="owner" class="team-invitations" :aria-label="t('team.invitations')">
              <div class="team-section-heading"><h3>{{ t('team.invitations') }}</h3><span class="team-muted">{{ snapshot.invitations.length }}</span></div>
              <form class="team-invite-form" @submit.prevent="mutate(() => teamAPI.invite(email.trim()), () => { email = '' })"><input v-model="email" class="input min-w-0 flex-1" type="email" required :aria-label="t('team.email')" :placeholder="t('team.email')" :disabled="busy" /><button class="btn btn-primary" :disabled="busy"><Icon name="mail" size="sm" aria-hidden="true" />{{ t('team.invite') }}</button></form>
              <div v-if="!snapshot.invitations.length" class="team-panel team-empty">{{ t('team.noInvitations') }}</div>
              <div v-for="invite in snapshot.invitations" :key="invite.id" class="team-panel team-invitation-row"><div class="min-w-0 break-all"><p class="team-member-name">{{ invite.email }}</p><p class="team-muted mt-1">{{ t(`team.invitationStatus.${invite.status}`) }} · {{ t('team.expires') }} {{ formatDateTime(invite.expires_at) }}</p></div><div class="team-toolbar"><button class="btn btn-secondary btn-sm" :disabled="busy" @click="mutate(() => teamAPI.resend(invite.id))"><Icon name="refresh" size="xs" aria-hidden="true" />{{ t('team.resend') }}</button><button class="btn btn-danger btn-sm" :disabled="busy" @click="mutate(() => teamAPI.revoke(invite.id))"><Icon name="x" size="xs" aria-hidden="true" />{{ t('team.revoke') }}</button></div></div>
            </section>
            <button v-else class="btn btn-danger" :disabled="busy" @click="ask(t('team.leave'), t('team.removeHint'), teamAPI.leave)">{{ t('team.leave') }}</button>
            <details v-if="owner && snapshot.usage" class="team-panel team-usage">
              <summary class="team-panel-heading"><h3>{{ t('team.teamUsage') }}</h3><Icon name="chevronDown" size="xs" class="team-chevron" aria-hidden="true" /></summary>
              <dl class="team-meters team-meters-total"><div v-for="period in [...windows, 'total'] as const" :key="period"><dt>{{ t(`team.${period}`) }}</dt><dd>{{ formatCurrency(snapshot.usage[period]) }}</dd></div></dl>
            </details>
          </section>
          <section v-else-if="tab === 'keys'" class="space-y-4">
            <p class="text-sm text-gray-500">{{ t('team.supportedEndpoints') }}</p>
            <p class="text-sm text-gray-500">{{ t(owner ? 'team.ownerKeysHint' : 'team.memberKeysHint') }}</p>
            <form v-if="!adminTeamId" class="flex flex-wrap gap-3" @submit.prevent="mutate(() => teamAPI.createKey(keyName.trim()), () => { keyName = '' })"><input v-model="keyName" class="input min-w-0 flex-1" required maxlength="100" :aria-label="t('team.keyName')" :placeholder="t('team.keyName')" :disabled="busy" /><button class="btn btn-primary" :disabled="busy || !keyName.trim()">{{ t('team.createKey') }}</button></form>
            <p v-if="!keys.length" class="card p-6 text-gray-500">{{ t('team.noKeys') }}</p>
            <article v-for="key in keys" :key="key.id" class="card space-y-3 p-5">
              <div class="flex flex-wrap justify-between gap-2"><h3 class="font-semibold">{{ key.name }}</h3><span class="text-sm">{{ t(`team.${key.status}`) }}</span></div>
              <p v-if="owner" class="break-all text-sm text-gray-500">{{ key.email }}</p>
              <code class="block break-all rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800">{{ revealed.includes(key.id) ? key.key : '••••••••••••••••' }}</code>
              <div class="flex flex-wrap gap-2"><button class="btn btn-secondary btn-sm" @click="revealed = revealed.includes(key.id) ? revealed.filter(id => id !== key.id) : [...revealed, key.id]">{{ t(revealed.includes(key.id) ? 'team.hide' : 'team.reveal') }}</button><button class="btn btn-secondary btn-sm" @click="copyToClipboard(key.key)">{{ t('team.copy') }}</button><button class="btn btn-secondary btn-sm" :disabled="busy" @click="editingKey = key; keyName = key.name">{{ t('team.rename') }}</button><button class="btn btn-secondary btn-sm" :disabled="busy" @click="mutate(() => teamAPI.updateKey(key.id, { status: key.status === 'active' ? 'disabled' : 'active' }))">{{ t(key.status === 'active' ? 'team.disable' : 'team.enable') }}</button><button class="btn btn-danger btn-sm" :disabled="busy" @click="ask(t('team.deleteKey'), t('team.deleteKeyHint'), () => teamAPI.deleteKey(key.id))">{{ t('team.deleteKey') }}</button></div>
              <p class="text-xs text-gray-500">{{ t('team.total') }}: {{ formatCurrency(key.quota_used) }} · {{ t('team.lastUsed') }}: {{ formatDateTime(key.last_used_at) }}</p>
            </article>
          </section>
          <section v-else-if="owner" class="space-y-6">
            <form class="card space-y-5 p-5" @submit.prevent="mutate(() => teamAPI.update({ name: name.trim(), group_ids: [...selected], default_limits: { ...defaults } }))">
              <fieldset :disabled="busy" class="space-y-5">
                <label class="block">{{ t('team.name') }}<input v-model="name" class="input mt-2" required maxlength="100" /></label>
                <div><h3 class="font-semibold">{{ t('team.defaultLimits') }}</h3><p class="mb-3 text-sm text-gray-500">{{ t('team.limitsHint') }}</p><div class="grid gap-3 sm:grid-cols-3"><label v-for="period in windows" :key="period">{{ t(`team.${period}`) }}<input v-model.number="defaults[period]" type="number" min="0" step="any" required class="input mt-1" /></label></div></div>
                <div><h3 class="font-semibold">{{ t('team.routing') }}</h3><p class="my-2 text-sm text-gray-500">{{ t('team.routingHint') }}</p><p v-if="!groups.length" class="text-sm text-amber-700">{{ t('team.noGroups') }}</p><label v-for="group in groups" :key="group.id" class="my-2 flex items-center gap-3"><input v-model="selected" type="checkbox" :value="group.id" /><span>{{ group.name }} <span class="text-sm text-gray-500">· {{ t(group.subscription_type === 'subscription' ? 'team.subscription' : 'team.balance') }} · ×{{ group.rate_multiplier }}</span></span></label></div>
                <ol class="space-y-2"><li v-for="(id, index) in selected" :key="id" class="flex items-center justify-between gap-3 rounded-lg border p-3 dark:border-dark-700"><span>{{ index + 1 }}. {{ groups.find(group => group.id === id)?.name || t('team.unavailableGroup', { id }) }}</span><div class="flex shrink-0 gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="busy || index === 0" :aria-label="t('team.moveUp')" @click="move(index, -1)">↑</button><button type="button" class="btn btn-secondary btn-sm" :disabled="busy || index === selected.length - 1" :aria-label="t('team.moveDown')" @click="move(index, 1)">↓</button><button type="button" class="btn btn-secondary btn-sm" :aria-label="t('team.removeGroup')" @click="selected = selected.filter(value => value !== id)">×</button></div></li></ol>
                <p v-if="!selected.length" class="text-sm text-amber-700">{{ t('team.emptyRouting') }}</p>
                <button class="btn btn-primary" :disabled="busy || !name.trim()">{{ t('team.save') }}</button>
              </fieldset>
            </form>
            <div class="card space-y-4 p-5"><h3 class="font-semibold">{{ t('team.lifecycle') }}</h3><p class="text-sm text-gray-500">{{ t('team.lifecycleHint') }}</p><div class="flex flex-wrap gap-3"><button class="btn btn-secondary" :disabled="busy" @click="mutate(() => teamAPI.update({ status: team?.status === 'active' ? 'paused' : 'active' }))">{{ t(team.status === 'active' ? 'team.pause' : 'team.resume') }}</button><button class="btn btn-danger" :disabled="busy" @click="ask(t('team.dissolve'), t('team.dissolveHint'), () => teamAPI.dissolve(confirmName), true)">{{ t('team.dissolve') }}</button></div></div>
          </section>
        </template>
        <p v-else class="text-gray-500">{{ t('team.noTeam') }}</p>
      </template>
    </div>
    <BaseDialog :show="showGuide" :title="t('team.guide')" @close="showGuide = false">
      <ol class="space-y-5 text-sm leading-relaxed">
        <li v-for="step in ['routing', 'members', 'keys'] as const" :key="step"><h3 class="mb-1 font-semibold">{{ t(`team.${step}`) }}</h3><p class="text-gray-500 dark:text-gray-400">{{ t(`team.guideSteps.${step}`) }}</p></li>
      </ol>
      <template #footer><button class="btn btn-secondary" @click="showGuide = false">{{ t('team.closeGuide') }}</button></template>
    </BaseDialog>
    <BaseDialog :show="!!confirmation" :title="confirmation?.title || ''" :close-on-escape="!busy" :close-on-click-outside="!busy" :show-close-button="!busy" @close="confirmation = null">
      <p class="text-sm">{{ confirmation?.message }}</p>
      <label v-if="confirmation?.dissolve" class="mt-4 block">{{ t('team.confirmName', { name: team?.name }) }}<input v-model="confirmName" class="input mt-2" :disabled="busy" autocomplete="off" /></label>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" :disabled="busy" @click="confirmation = null">{{ t('team.cancel') }}</button><button class="btn btn-danger" :disabled="busy || (!!confirmation?.dissolve && confirmName !== team?.name)" @click="confirm">{{ t('team.confirm') }}</button></div></template>
    </BaseDialog>
    <BaseDialog :show="!!editingMember" :title="t('team.editLimits')" @close="!busy && (editingMember = null)">
      <form class="space-y-4" @submit.prevent="mutate(() => teamAPI.limits(editingMember!.user_id, { ...memberLimits }), () => { editingMember = null })"><p class="break-all">{{ editingMember?.email }}</p><p class="text-sm text-gray-500">{{ t('team.limitsHint') }}</p><label v-for="period in windows" :key="period" class="block">{{ t(`team.${period}`) }}<input v-model.number="memberLimits[period]" class="input mt-1" type="number" min="0" step="any" required :disabled="busy" /></label><button class="btn btn-primary" :disabled="busy">{{ t('team.save') }}</button></form>
    </BaseDialog>
    <BaseDialog :show="!!editingKey" :title="t('team.rename')" @close="!busy && (editingKey = null)"><form class="space-y-4" @submit.prevent="saveKey"><label class="block">{{ t('team.keyName') }}<input v-model="keyName" class="input mt-2" required maxlength="100" :disabled="busy" /></label><button class="btn btn-primary" :disabled="busy || !keyName.trim()">{{ t('team.save') }}</button></form></BaseDialog>
  </AppLayout>
</template>

<style scoped>
.team-layout {
  --team-bg: #fafbfc;
  --team-surface: #fff;
  --team-border: #e5eaf0;
  --team-text: #202327;
  --team-muted: #516d87;
  --team-accent: #0aa7e0;
  background: var(--team-bg);
  color: var(--team-text);
}
.team-layout :deep(.bg-mesh-gradient) { display: none; }
.team-layout :deep(main) { padding: 20px 28px 40px; }
.team-layout :deep(.sidebar-link-active) { color: var(--team-accent); background: color-mix(in srgb, var(--team-accent) 12%, transparent); }
.team-page { display: flex; flex-direction: column; gap: 20px; font-size: 14px; line-height: 1.5; }
.team-heading { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.team-heading h1 { font-size: 22px; font-weight: 700; line-height: 1.35; }
.team-heading p { margin-top: 4px; color: var(--team-muted); font-size: 13px; }
.team-summary { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.team-name { overflow-wrap: anywhere; font-size: 21px; font-weight: 600; line-height: 1.3; }
.team-muted { color: var(--team-muted); }
.team-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; flex-shrink: 0; }
.team-badge { display: inline-flex; align-items: center; border-radius: 20px; padding: 1px 9px; font-size: 12px; font-weight: 500; line-height: 1.5; white-space: nowrap; }
.team-badge-active { background: #d9f9ed; color: #059669; }
.team-badge-paused { background: #fef3c7; color: #b45309; }
.team-badge-role { background: #def4fc; color: #028ccc; }
.team-tabs { display: flex; gap: 4px; border-bottom: 1px solid var(--team-border); }
.team-tabs button { margin-bottom: -1px; border-bottom: 2px solid transparent; padding: 10px 15px; color: var(--team-muted); font-size: 13px; transition: color 150ms; }
.team-tabs button:hover, .team-tabs .is-active { color: var(--team-accent); }
.team-tabs .is-active { border-bottom-color: var(--team-accent); }
.team-overview { display: flex; flex-direction: column; align-items: stretch; gap: 24px; }
.team-panel, .team-page :deep(.card) { background: var(--team-surface); border: 1px solid var(--team-border); border-radius: 6px; box-shadow: none; }
.team-panel-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 18px; }
.team-panel-heading h3, .team-section-heading h3 { font-size: 14px; font-weight: 600; }
.team-panel-heading .team-muted { font-size: 13px; }
.team-member { border-top: 1px solid var(--team-border); }
.team-member-summary { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 20px 18px; cursor: pointer; list-style: none; }
summary::-webkit-details-marker { display: none; }
.team-member-name { font-size: 15px; font-weight: 500; overflow-wrap: anywhere; }
.team-detail-toggle { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--team-muted); flex-shrink: 0; }
.team-member-summary:hover .team-detail-toggle { color: var(--team-accent); }
.team-member-details { padding: 0 18px 18px; }
.team-chevron { transition: transform 150ms; }
details[open] > summary .team-chevron { transform: rotate(180deg); }
.team-meters { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 20px; padding-top: 16px; border-top: 1px solid var(--team-border); }
.team-meters dt { color: var(--team-muted); font-size: 12px; }
.team-meters dd { margin-top: 5px; font-size: 14px; font-variant-numeric: tabular-nums; }
.team-invitations { display: flex; flex-direction: column; gap: 16px; }
.team-section-heading { display: flex; align-items: center; justify-content: space-between; }
.team-invite-form { display: flex; gap: 10px; }
.team-empty { display: grid; place-items: center; min-height: 96px; color: var(--team-muted); font-size: 13px; }
.team-invitation-row { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 16px; padding: 20px 18px; }
.team-usage > summary { cursor: pointer; list-style: none; color: var(--team-muted); }
.team-meters-total { grid-template-columns: repeat(4, minmax(0, 1fr)); margin: 0 18px; padding-bottom: 18px; }
.team-page :deep(.btn) { min-height: 34px; border-radius: 5px; padding: 7px 13px; font-size: 13px; font-weight: 400; line-height: 18px; gap: 7px; box-shadow: none; transform: none; }
.team-page :deep(.btn-secondary) { background: var(--team-surface); border: 1px solid var(--team-border); color: var(--team-muted); }
.team-page :deep(.btn-secondary:hover:not(:disabled)) { background: var(--team-bg); border-color: #c4d4df; }
.team-page :deep(.btn-primary) { background: var(--team-accent); border: 1px solid var(--team-accent); color: white; }
.team-page :deep(.btn-primary:hover:not(:disabled)) { background: #0595cc; }
.team-page :deep(.btn-danger) { background: #e72329; border: 1px solid #e72329; color: white; }
.team-page :deep(.btn-sm) { min-height: 29px; padding: 4px 10px; font-size: 12px; }
.team-page :deep(.input) { min-height: 34px; border-radius: 5px; border: 1px solid var(--team-border); background: var(--team-surface); padding: 7px 12px; font-size: 13px; color: var(--team-text); box-shadow: none; }
.team-page :deep(.input::placeholder) { color: #96a9bb; }
.team-page input[type='checkbox'] { accent-color: var(--team-accent); }
.team-page :deep(.input:focus) { border-color: var(--team-accent); outline: 2px solid color-mix(in srgb, var(--team-accent) 15%, transparent); }
.team-page :deep(button:focus-visible), .team-page summary:focus-visible { outline: 2px solid var(--team-accent); outline-offset: 3px; }
.dark .team-layout { --team-bg: #101820; --team-surface: #16212c; --team-border: #2b3c4b; --team-text: #e5edf4; --team-muted: #a1b6c8; }
.dark .team-badge-active { background: #113d32; color: #6ee7b7; }
.dark .team-badge-role { background: #11374a; color: #7dd3fc; }
.dark .team-badge-paused { background: #45371d; color: #fcd34d; }
@media (max-width: 640px) {
  .team-layout :deep(main) { padding: 20px 16px 32px; }
  .team-page { gap: 18px; }
  .team-summary { align-items: flex-start; flex-direction: column; gap: 14px; }
  .team-member-summary { align-items: flex-start; flex-direction: column; gap: 12px; }
  .team-meters, .team-meters-total { grid-template-columns: 1fr; gap: 16px; }
  .team-invite-form { flex-wrap: wrap; }
  .team-invite-form .input { flex-basis: 100%; }
  .team-invite-form .btn { margin-left: auto; }
}
@media (prefers-reduced-motion: reduce) { .team-chevron, .team-tabs button { transition: none; } }
</style>
