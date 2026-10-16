<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <header class="flex items-center justify-between gap-4">
        <div><h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('luckyDraw.title') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('luckyDraw.description') }}</p></div>
        <button class="btn btn-secondary" :disabled="loading || busy" @click="load">{{ t('luckyDraw.refresh') }}</button>
      </header>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <p v-if="loading && !snapshot" role="status" class="card p-10 text-center">{{ t('luckyDraw.loading') }}</p>
      <template v-if="snapshot">
        <div class="grid gap-6 lg:grid-cols-3">
          <section class="card overflow-hidden lg:col-span-2">
            <div class="flex flex-wrap justify-center gap-2 border-b border-gray-100 p-4 dark:border-dark-700">
              <button v-for="item in modes" :key="item.value" class="btn" :class="mode === item.value ? 'btn-primary' : 'btn-secondary'" :disabled="busy" :aria-pressed="mode === item.value" @click="changeMode(item.value)">{{ t(item.label) }}</button>
            </div>
            <LuckyDrawStage :mode="mode" :prizes="stagePrizes" :busy="busy" :can-draw="canDraw" :result="result" :selected="selected" @draw="draw" />
            <div class="px-6 pb-6 text-center" aria-live="polite">
              <p v-if="result && !busy" class="text-lg font-semibold text-violet-600 dark:text-violet-300">{{ result.prize_amount > 0 ? t('luckyDraw.won', { prize: result.prize_name, amount: result.prize_amount.toFixed(2) }) : t('luckyDraw.noWin') }}</p>
              <p v-else class="text-sm text-gray-500">{{ busy ? t('luckyDraw.drawing') : !snapshot.config.enabled ? t('luckyDraw.paused') : mode === 'cards' ? t('luckyDraw.selectCard') : t('luckyDraw.tryLuck') }}</p>
              <button v-if="pending" class="btn btn-secondary mt-3" :disabled="busy || loading" @click="draw(selected)">{{ t('luckyDraw.recover') }}</button>
            </div>
          </section>
          <aside class="space-y-5">
            <section class="card p-6">
              <h2 class="text-sm text-gray-500">{{ t('luckyDraw.chances') }}</h2>
              <p class="my-3 text-5xl font-bold tabular-nums text-violet-600 dark:text-violet-400">{{ snapshot.chances }}</p>
              <p class="text-sm leading-6 text-gray-500">{{ t('luckyDraw.rechargeRule', { amount: snapshot.config.recharge_per_draw.toFixed(2) }) }}</p>
              <p class="mt-2 text-xs text-gray-500">{{ t('luckyDraw.recharged', { amount: snapshot.total_recharged.toFixed(2), used: snapshot.draws_used }) }}</p>
              <router-link to="/purchase" class="btn btn-primary mt-4 w-full">{{ t('luckyDraw.recharge') }}</router-link>
            </section>
            <section class="card p-6">
              <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.guarantee') }}</h2>
              <p class="mt-3 text-sm leading-6 text-gray-500">{{ snapshot.config.guarantee_after ? t('luckyDraw.guaranteeRule', { count: snapshot.config.guarantee_after, remaining: Math.max(1, snapshot.config.guarantee_after - snapshot.misses) }) : t('luckyDraw.noGuarantee') }}</p>
            </section>
          </aside>
        </div>
        <section class="card p-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.prizes') }}</h2>
          <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
            <div v-for="(prize, i) in snapshot.config.prizes" :key="i" class="rounded-xl bg-violet-50 p-4 dark:bg-violet-950/25">
              <p class="break-words font-semibold text-violet-800 dark:text-violet-200">{{ prize.name }}</p>
              <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ prize.amount > 0 ? `$${prize.amount.toFixed(2)}` : t('luckyDraw.noPrize') }}</p>
              <p class="mt-1 text-xs text-gray-500">{{ t('luckyDraw.probability', { value: (prize.weight / 100).toFixed(2) }) }}</p>
            </div>
          </div>
          <p class="mt-4 text-xs leading-6 text-gray-500">{{ t('luckyDraw.rules') }}</p>
        </section>
        <section class="card overflow-hidden">
          <h2 class="p-6 text-lg font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.history') }}</h2>
          <div class="overflow-x-auto"><table v-if="snapshot.history.length" class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-gray-500 dark:bg-dark-800"><tr><th class="px-6 py-3">{{ t('luckyDraw.time') }}</th><th class="px-6 py-3">{{ t('luckyDraw.prize') }}</th><th class="px-6 py-3">{{ t('luckyDraw.status') }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="record in snapshot.history" :key="record.id"><td class="whitespace-nowrap px-6 py-4 text-gray-500">{{ new Date(record.created_at).toLocaleString(locale) }}</td><td class="px-6 py-4 text-gray-900 dark:text-white">{{ record.prize_name }} <span v-if="record.guaranteed" class="ml-2 text-xs text-amber-600">{{ t('luckyDraw.guarantee') }}</span></td><td class="px-6 py-4 text-emerald-600">{{ record.prize_amount > 0 ? t('luckyDraw.credited', { amount: record.prize_amount.toFixed(2) }) : t('luckyDraw.noPrize') }}</td></tr></tbody>
          </table><p v-else class="p-8 text-center text-sm text-gray-500">{{ t('luckyDraw.empty') }}</p></div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LuckyDrawStage from '@/components/lottery/LuckyDrawStage.vue'
import { luckyDrawAPI, type LuckyDrawSnapshot, type LuckyDrawRecord, type LuckyDrawRequest, type LuckyDrawPrize } from '@/api/luckyDraw'
import { useAuthStore } from '@/stores/auth'
const { t, locale } = useI18n()
const auth = useAuthStore()
const snapshot = ref<LuckyDrawSnapshot | null>(null)
const mode = ref<'cards' | 'wheel' | 'roller'>('cards')
const modes = [{ value: 'cards', label: 'luckyDraw.cards' }, { value: 'wheel', label: 'luckyDraw.wheel' }, { value: 'roller', label: 'luckyDraw.roller' }] as const
const result = ref<LuckyDrawRecord | null>(null)
const stagePrizes = ref<LuckyDrawPrize[]>([])
const selected = ref(0)
const busy = ref(false)
const loading = ref(false)
const error = ref('')
const storageKey = `lucky-draw-pending:${auth.user?.id}`
const pending = ref<LuckyDrawRequest | null>(readPending())
function readPending(): LuckyDrawRequest | null {
  try {
    const value = JSON.parse(localStorage.getItem(storageKey) || 'null')
    return value && typeof value.request_id === 'string' && Number.isInteger(value.config_version) ? value : null
  } catch { return null }
}
function remember(value: LuckyDrawRequest | null) {
  pending.value = value
  try { if (value) localStorage.setItem(storageKey, JSON.stringify(value)); else localStorage.removeItem(storageKey) } catch { /* Retry still works during this visit. */ }
}
const canDraw = computed(() => !loading.value && (!!pending.value || !!snapshot.value?.config.enabled && snapshot.value.chances > 0))
function changeMode(value: typeof mode.value) {
  mode.value = value
  result.value = null
  if (snapshot.value) stagePrizes.value = snapshot.value.config.prizes
}
async function load() {
  if (loading.value) return
  loading.value = true
  try {
    snapshot.value = await luckyDrawAPI.get()
    if (!result.value) stagePrizes.value = snapshot.value.config.prizes
    error.value = ''
  } catch { error.value = t('luckyDraw.loadFailed') }
  finally { loading.value = false }
}
async function draw(card: number) {
  if (busy.value || !canDraw.value || !snapshot.value) return
  busy.value = true; error.value = ''; result.value = null; selected.value = card
  stagePrizes.value = snapshot.value.config.prizes
  try {
    if (!pending.value) {
      const bytes = crypto.getRandomValues(new Uint8Array(16))
      bytes[6] = (bytes[6]! & 15) | 64
      bytes[8] = (bytes[8]! & 63) | 128
      const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')
      const requestID = `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
      remember({ request_id: requestID, config_version: snapshot.value.config.version })
    }
    const record = await luckyDrawAPI.draw(pending.value!)
    remember(null)
    if (record.config_version !== snapshot.value.config.version) mode.value = 'cards'
    result.value = record
    await new Promise(resolve => setTimeout(resolve, mode.value === 'cards' ? 600 : 1900))
    await load()
    if (record.prize_amount > 0) void auth.refreshUser().catch(() => {})
  } catch (err: unknown) {
    const e = err as { reason?: string; status?: number; response?: { status?: number } }
    const reason = e.reason
    if (reason === 'LUCKY_DRAW_CHANGED' || reason === 'LUCKY_DRAW_NO_CHANCES' || reason === 'LUCKY_DRAW_PAUSED' || reason === 'INVALID_LUCKY_DRAW_REQUEST') remember(null)
    await load()
    error.value = reason === 'LUCKY_DRAW_CHANGED' ? t('luckyDraw.changed') : reason === 'LUCKY_DRAW_NO_CHANCES' ? t('luckyDraw.noChances') : reason === 'LUCKY_DRAW_PAUSED' ? t('luckyDraw.paused') : t('luckyDraw.drawFailed')
  } finally { busy.value = false }
}
onMounted(load)
</script>
