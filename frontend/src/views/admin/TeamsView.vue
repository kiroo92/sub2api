<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import EmailTemplateEditor from './settings/EmailTemplateEditor.vue'
import { adminTeamsAPI, type TeamAdminItem } from '@/api/team'
import { formatCurrency } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const app = useAppStore()
const tab = ref<'teams' | 'config'>('teams')
const search = ref('')
const status = ref('')
const page = ref(1)
const total = ref(0)
const items = ref<TeamAdminItem[]>([])
const busy = ref(false)
const error = ref('')
const frontendURL = ref('')
function report(reason: unknown) { error.value = extractApiErrorMessage(reason, t('team.failed')); app.showError(error.value) }
async function load() {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    const result = await adminTeamsAPI.list({ search: search.value.trim(), status: status.value, page: page.value, page_size: 20 })
    items.value = result.items; total.value = result.total
  } catch (reason) { report(reason) } finally { busy.value = false }
}
async function configure() {
  if (busy.value) return
  tab.value = 'config'; busy.value = true; error.value = ''
  try { frontendURL.value = (await adminTeamsAPI.config()).frontend_url } catch (reason) { report(reason) } finally { busy.value = false }
}
async function save() {
  busy.value = true; error.value = ''
  try { await adminTeamsAPI.saveConfig(frontendURL.value.trim()); app.showSuccess(t('team.saved')) } catch (reason) { report(reason) } finally { busy.value = false }
}
onMounted(load)
</script>

<template>
  <AppLayout>
    <div class="space-y-5">
      <h1 class="text-2xl font-semibold">{{ t('team.adminTitle') }}</h1>
      <nav class="flex gap-6 border-b dark:border-dark-700">
        <button class="border-b-2 py-3" :class="tab === 'teams' ? 'border-primary-500 text-primary-600' : 'border-transparent'" :disabled="busy" @click="tab = 'teams'; load()">{{ t('team.allTeams') }}</button>
        <button class="border-b-2 py-3" :class="tab === 'config' ? 'border-primary-500 text-primary-600' : 'border-transparent'" :disabled="busy" @click="configure">{{ t('team.invitationConfig') }}</button>
      </nav>
      <p v-if="error" role="alert" class="text-red-600">{{ error }}</p>
      <template v-if="tab === 'teams'">
        <form class="flex flex-wrap gap-3" @submit.prevent="page = 1; load()">
          <input v-model="search" class="input min-w-0 flex-1" :aria-label="t('team.search')" :placeholder="t('team.search')" maxlength="200" />
          <select v-model="status" class="input w-auto" :aria-label="t('team.lifecycle')"><option value="">{{ t('team.allStatuses') }}</option><option value="active">{{ t('team.active') }}</option><option value="paused">{{ t('team.paused') }}</option></select>
          <button class="btn btn-primary" :disabled="busy">{{ t('team.searchAction') }}</button>
          <button type="button" class="btn btn-secondary" :title="t('team.refresh')" :aria-label="t('team.refresh')" :disabled="busy" @click="load"><Icon name="refresh" size="sm" /></button>
        </form>
        <div class="overflow-x-auto" :aria-busy="busy">
          <table class="w-full text-left text-sm"><thead><tr class="border-b dark:border-dark-700"><th class="p-3">{{ t('team.name') }}</th><th class="p-3">{{ t('team.owner') }}</th><th class="p-3">{{ t('team.lifecycle') }}</th><th class="p-3">{{ t('team.members') }}</th><th class="p-3">{{ t('team.total') }}</th><th class="p-3">{{ t('team.manage') }}</th></tr></thead>
            <tbody><tr v-for="item in items" :key="item.id" class="border-b dark:border-dark-700"><td class="max-w-64 break-words p-3">{{ item.name }}</td><td class="max-w-64 break-all p-3">{{ item.owner_email }}</td><td class="p-3">{{ t(`team.${item.status}`) }}</td><td class="p-3">{{ item.member_count }}</td><td class="p-3 tabular-nums">{{ formatCurrency(item.total_usage) }}</td><td class="p-3"><RouterLink :to="`/admin/teams/${item.id}`" class="text-primary-600">{{ t('team.manage') }}</RouterLink></td></tr></tbody>
          </table>
          <p v-if="!busy && !items.length" class="py-8 text-center text-gray-500">{{ t('team.noTeams') }}</p>
        </div>
        <div class="flex items-center justify-end gap-3"><span class="text-sm">{{ t('team.pagination', { page, total }) }}</span><button class="btn btn-secondary" :disabled="busy || page <= 1" @click="page--; load()">{{ t('team.previous') }}</button><button class="btn btn-secondary" :disabled="busy || page * 20 >= total" @click="page++; load()">{{ t('team.next') }}</button></div>
      </template>
      <template v-else>
        <form class="max-w-2xl space-y-4 py-3" @submit.prevent="save">
          <label class="block font-medium">{{ t('team.invitationDomain') }}<input v-model="frontendURL" type="url" class="input mt-2" placeholder="https://example.com" :disabled="busy" /></label>
          <p class="text-sm text-gray-500">{{ t('team.domainHint') }}</p>
          <button class="btn btn-primary" :disabled="busy">{{ t('team.save') }}</button>
        </form>
        <EmailTemplateEditor event-filter="team.invitation" />
      </template>
    </div>
  </AppLayout>
</template>
