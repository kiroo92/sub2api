<template>
  <section class="card space-y-5 p-6" aria-labelledby="draw-stats-title">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div><h2 id="draw-stats-title" class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.statsTitle') }}</h2><p v-if="data" class="mt-1 text-xs text-gray-500">{{ t('luckyDraw.startsAt', { time: formatStart(data.recharge_starts_at) }) }}</p></div>
      <button class="btn btn-secondary" :disabled="loading" @click="load">{{ t('luckyDraw.refresh') }}</button>
    </div>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <p v-if="loading && !data" role="status" class="text-sm text-gray-500">{{ t('luckyDraw.loading') }}</p>
    <template v-if="data">
      <dl class="grid grid-cols-2 gap-3 lg:grid-cols-6">
        <div v-for="item in summary" :key="item.key" class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800"><dt class="text-xs text-gray-500">{{ t(item.label) }}</dt><dd class="mt-2 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.money ? '¥' + Number(data.summary[item.key]).toFixed(2) : data.summary[item.key] }}</dd></div>
      </dl>
      <p class="text-xs text-gray-500">{{ t('luckyDraw.summaryHint') }}</p>
      <form ref="grantForm" class="space-y-3 rounded-xl border border-amber-200 bg-amber-50/40 p-4 dark:border-amber-900 dark:bg-amber-950/10" @submit.prevent="grantChances">
        <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.grantTitle') }}</h3>
        <p class="text-xs text-gray-500">{{ t('luckyDraw.grantHint') }}</p>
        <div class="grid gap-3 sm:grid-cols-3"><label class="input-label">{{ t('luckyDraw.userID') }}<input v-model.number="grantUser" type="number" min="1" step="1" required class="input mt-1" :disabled="grantBusy || !!pendingGrant" /></label><label class="input-label">{{ t('luckyDraw.grantCount') }}<input v-model.number="grantCount" type="number" min="1" max="10000" step="1" required class="input mt-1" :disabled="grantBusy || !!pendingGrant" /></label><label class="input-label">{{ t('luckyDraw.note') }}<input v-model="grantNote" maxlength="500" class="input mt-1" :disabled="grantBusy || !!pendingGrant" /></label></div>
        <p v-if="grantError" role="alert" class="text-xs text-red-600">{{ grantError }}</p><p v-if="grantSaved" role="status" class="text-xs text-emerald-600">{{ grantSaved }}</p>
        <button class="btn btn-primary" type="submit" :disabled="grantBusy || loading">{{ grantBusy ? t('luckyDraw.saving') : pendingGrant ? t('luckyDraw.recoverGrant') : t('luckyDraw.grantTitle') }}</button>
      </form>
      <form class="flex flex-wrap gap-3" @submit.prevent="searchUsers">
        <input v-model="search" :aria-label="t('luckyDraw.searchUsers')" :placeholder="t('luckyDraw.searchUsers')" maxlength="200" class="input min-w-48 flex-1" />
        <button class="btn btn-secondary" :disabled="loading" type="submit">{{ t('luckyDraw.search') }}</button>
      </form>
      <div class="overflow-x-auto">
        <table class="w-full whitespace-nowrap text-left text-sm">
          <thead class="text-xs text-gray-500"><tr><th class="p-3">{{ t('luckyDraw.user') }}</th><th class="p-3">{{ t('luckyDraw.eligibleRecharge') }}</th><th class="p-3">{{ t('luckyDraw.earned') }}</th><th class="p-3">{{ t('luckyDraw.manualChances') }}</th><th class="p-3">{{ t('luckyDraw.used') }}</th><th class="p-3">{{ t('luckyDraw.remaining') }}</th><th class="p-3">{{ t('luckyDraw.wins') }}</th><th class="p-3">{{ t('luckyDraw.awarded') }}</th><th class="p-3">{{ t('luckyDraw.grantTitle') }}</th></tr></thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="user in data.users" :key="user.user_id" class="text-gray-900 dark:text-gray-200"><td class="p-3">#{{ user.user_id }} · {{ user.email }}</td><td class="p-3">¥{{ user.recharge_amount.toFixed(2) }}</td><td class="p-3">{{ user.chances_earned }}</td><td class="p-3">{{ user.manual_chances }}</td><td class="p-3">{{ user.draws_used }}</td><td class="p-3">{{ user.chances }}</td><td class="p-3">{{ user.wins }}</td><td class="p-3 text-emerald-600">¥{{ user.prize_amount.toFixed(2) }}</td><td class="p-3"><button class="text-xs font-medium text-primary-600 hover:underline" :disabled="grantBusy || !!pendingGrant" @click="selectGrantUser(user.user_id)">{{ t('luckyDraw.grantTitle') }}</button></td></tr></tbody>
        </table>
        <p v-if="!data.users.length" class="p-5 text-center text-sm text-gray-500">{{ t('luckyDraw.noUsers') }}</p>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500"><span>{{ t('luckyDraw.pageInfo', { page: data.page, total: data.total }) }}</span><div class="flex gap-2"><button class="btn btn-secondary" :disabled="loading || page <= 1" @click="page--; load()">{{ t('luckyDraw.previous') }}</button><button class="btn btn-secondary" :disabled="loading || data.page * data.page_size >= data.total" @click="page++; load()">{{ t('luckyDraw.next') }}</button></div></div>
      <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.prizeStats') }}</h3>
      <div class="overflow-x-auto"><table v-if="data.prizes.length" class="w-full text-left text-sm"><thead class="text-xs text-gray-500"><tr><th class="p-3">{{ t('luckyDraw.prize') }}</th><th class="p-3">{{ t('luckyDraw.drawCount') }}</th><th class="p-3">{{ t('luckyDraw.awarded') }}</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="prize in data.prizes" :key="prize.name + ':' + prize.amount" class="text-gray-900 dark:text-gray-200"><td class="p-3">{{ prize.name }} <span class="text-xs text-gray-500">¥{{ prize.amount.toFixed(2) }}</span></td><td class="p-3">{{ prize.draws }}</td><td class="p-3">¥{{ prize.total_awarded.toFixed(2) }}</td></tr></tbody></table><p v-else class="text-sm text-gray-500">{{ t('luckyDraw.empty') }}</p></div>
      <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.grantHistory') }}</h3>
      <div class="overflow-x-auto"><table v-if="data.grants?.length" class="w-full whitespace-nowrap text-left text-xs"><thead class="text-gray-500"><tr><th class="p-3">{{ t('luckyDraw.time') }}</th><th class="p-3">{{ t('luckyDraw.userID') }}</th><th class="p-3">{{ t('luckyDraw.adminID') }}</th><th class="p-3">{{ t('luckyDraw.grantCount') }}</th><th class="p-3">{{ t('luckyDraw.note') }}</th></tr></thead><tbody><tr v-for="grant in data.grants" :key="grant.id" class="text-gray-900 dark:text-gray-200"><td class="p-3">{{ new Date(grant.created_at).toLocaleString() }}</td><td class="p-3">#{{ grant.user_id }}</td><td class="p-3">#{{ grant.admin_id }}</td><td class="p-3">+{{ grant.chances }}</td><td class="max-w-xs whitespace-normal break-words p-3">{{ grant.note || '—' }}</td></tr></tbody></table><p v-else class="text-xs text-gray-500">{{ t('luckyDraw.noGrants') }}</p></div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { luckyDrawAPI, type LuckyDrawAdminStats, type LuckyDrawTotals } from '@/api/luckyDraw'
import { luckyDrawRequestID } from '@/utils/luckyDraw'
import { useAuthStore } from '@/stores/auth'
const { t } = useI18n()
const data = ref<LuckyDrawAdminStats | null>(null)
const loading = ref(false)
const error = ref('')
const search = ref('')
const activeSearch = ref('')
const page = ref(1)
const summary: { key: keyof LuckyDrawTotals; label: string; money?: boolean }[] = [
  { key: 'recharge_amount', label: 'luckyDraw.eligibleRecharge', money: true },
  { key: 'chances_earned', label: 'luckyDraw.earned' }, { key: 'manual_chances', label: 'luckyDraw.manualChances' }, { key: 'draws_used', label: 'luckyDraw.used' },
  { key: 'chances', label: 'luckyDraw.remaining' }, { key: 'wins', label: 'luckyDraw.wins' },
  { key: 'prize_amount', label: 'luckyDraw.awarded', money: true }
]
const formatStart = (value: string) => new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' }) + ' (UTC+8)'
async function load() {
  if (loading.value) return
  loading.value = true; error.value = ''
  try { data.value = await luckyDrawAPI.stats(activeSearch.value, page.value) }
  catch { error.value = t('luckyDraw.loadFailed'); if (data.value) page.value = data.value.page }
  finally { loading.value = false }
}
function searchUsers() { if (loading.value) return; page.value = 1; activeSearch.value = search.value.trim(); void load() }
type GrantInput = { user_id: number; count: number; note: string; request_id: string }
const grantKey = `lucky-draw-grant:${useAuthStore().user?.id}`
function readGrant(): GrantInput | null {
  try { const value = JSON.parse(localStorage.getItem(grantKey) || 'null'); return value && Number.isInteger(value.user_id) && Number.isInteger(value.count) && typeof value.request_id === 'string' && typeof value.note === 'string' ? value : null } catch { return null }
}
const pendingGrant = ref<GrantInput | null>(readGrant())
const grantForm = ref<HTMLFormElement | null>(null)
const grantUser = ref(pendingGrant.value?.user_id ?? 0)
const grantCount = ref(pendingGrant.value?.count ?? 10)
const grantNote = ref(pendingGrant.value?.note ?? '')
const grantBusy = ref(false), grantError = ref(''), grantSaved = ref('')
function rememberGrant(value: GrantInput | null) { pendingGrant.value = value; try { if (value) localStorage.setItem(grantKey, JSON.stringify(value)); else localStorage.removeItem(grantKey) } catch { /* Retry remains available during this visit. */ } }
function selectGrantUser(id: number) { grantUser.value = id; grantSaved.value = ''; grantForm.value?.scrollIntoView?.({ block: 'center' }) }
async function grantChances() {
  if (grantBusy.value || loading.value) return
  grantBusy.value = true; grantError.value = ''; grantSaved.value = ''
  try {
    if (!pendingGrant.value) rememberGrant({ user_id: grantUser.value, count: grantCount.value, note: grantNote.value.trim(), request_id: luckyDrawRequestID() })
    const result = await luckyDrawAPI.grant(pendingGrant.value!)
    rememberGrant(null); grantSaved.value = t('luckyDraw.granted', { user: result.user_id, count: result.chances }); await load()
  } catch (err: unknown) {
    const error = err as { status?: number; reason?: string; message?: string }
    if (error.status === 400 || error.status === 404 || error.reason === 'LUCKY_DRAW_REQUEST_CONFLICT') { rememberGrant(null); grantError.value = error.message || t('luckyDraw.grantFailed') }
    else grantError.value = t('luckyDraw.grantUncertain')
  } finally { grantBusy.value = false }
}
onMounted(load)
</script>
