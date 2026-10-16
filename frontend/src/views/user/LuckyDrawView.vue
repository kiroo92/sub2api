<template>
  <AppLayout>
    <div class="lucky-page mx-auto max-w-6xl space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-4"><div><p class="lucky-eyebrow">{{ t('luckyDraw.tryLuck') }}</p><h1 class="mt-1 text-3xl font-bold text-gray-900 dark:text-white">{{ t('luckyDraw.title') }}</h1><p v-if="snapshot" class="mt-2 text-sm text-gray-500">{{ t('luckyDraw.rechargeRule', { amount: snapshot.config.recharge_per_draw.toFixed(2) }) }}</p></div><div v-if="snapshot" class="top-prize"><span>{{ t('luckyDraw.highestPrize') }}</span><strong>¥{{ highestPrize.toFixed(0) }}</strong></div><button class="btn btn-secondary" :disabled="loading || busy" @click="load">{{ t('luckyDraw.refresh') }}</button></header>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <p v-if="loading && !snapshot" role="status" class="card p-10 text-center">{{ t('luckyDraw.loading') }}</p>
      <template v-if="snapshot">
        <div class="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_290px]">
          <section class="lucky-arena" :class="{ 'skip-animation': skipAnimation }">
            <div class="arena-toolbar"><div class="flex flex-wrap gap-1"><button v-for="item in modes" :key="item.value" class="mode-button" :class="{ active: mode === item.value }" :disabled="busy" :aria-pressed="mode === item.value" @click="changeMode(item.value)">{{ t(item.label) }}</button></div><label class="demo-switch"><input v-model="demo" type="checkbox" :disabled="busy || !!pending" @change="clearResults" />{{ t('luckyDraw.demo') }}</label></div>
            <p v-if="demo" class="demo-notice" role="status">{{ t('luckyDraw.demoHint') }}</p>
            <div class="arena-content">
              <p class="mb-2 text-center text-sm text-amber-900/70 dark:text-amber-200/70">{{ demo ? t('luckyDraw.demo') : !snapshot.config.enabled ? t('luckyDraw.paused') : mode === 'cards' ? t('luckyDraw.selectCard') : t('luckyDraw.tryLuck') }}</p>
              <LuckyDrawStage :mode="mode" :prizes="stagePrizes" :busy="busy" :can-draw="canDraw" :result="result" :selected="selected" @draw="draw" />
              <div class="results-area" aria-live="polite">
                <p v-if="busy" class="text-center text-sm font-medium text-amber-800 dark:text-amber-200">{{ t('luckyDraw.drawing') }}</p>
                <template v-else-if="results.length">
                  <p class="mb-3 text-center text-sm font-semibold text-amber-900 dark:text-amber-200">{{ demo ? t('luckyDraw.demoComplete', { count: results.length }) : results.length > 1 ? t('luckyDraw.batchComplete', { count: results.length, amount: batchTotal.toFixed(2) }) : result && result.prize_amount > 0 ? t('luckyDraw.won', { prize: result.prize_name, amount: result.prize_amount.toFixed(2) }) : t('luckyDraw.noWin') }}</p>
                  <div :class="results.length > 1 ? 'grid grid-cols-2 gap-2 sm:grid-cols-5' : ''"><LuckyDrawReveal v-for="(record, i) in results" :key="record.id + ':' + i" :record="record" :demo="demo" :compact="results.length > 1" /></div>
                </template>
              </div>
              <button v-if="pending" class="btn btn-secondary mx-auto mt-3 block" :disabled="busy || loading" @click="draw(selected)">{{ t('luckyDraw.recover') }}</button>
              <button v-if="results.length && !busy" class="mx-auto mt-3 block text-xs font-semibold text-amber-700 hover:underline" @click="showResult = true">{{ t('luckyDraw.viewResult') }}</button>
            </div>
            <div class="draw-controls"><span class="text-xs font-semibold">{{ t('luckyDraw.drawCountSelect') }}</span><div class="flex gap-1"><button v-for="count in drawCounts" :key="count" class="count-button" :class="{ active: drawCount === count }" :disabled="busy || !!pending" :aria-pressed="drawCount === count" @click="drawCount = count">{{ t('luckyDraw.times', { count }) }}</button></div><label class="flex items-center gap-2 text-xs"><input v-model="skipAnimation" type="checkbox" :disabled="busy" />{{ t('luckyDraw.skipAnimation') }}</label><button class="draw-button" :disabled="busy || !canDraw" @click="draw(0)">{{ busy ? t('luckyDraw.drawing') : t('luckyDraw.drawTimes', { count: pending?.count || drawCount }) }}</button></div>
          </section>
          <aside class="space-y-4">
            <section class="lucky-panel p-5"><div class="flex items-center gap-4"><div class="guarantee-ring" :style="{ '--progress': `${snapshot.config.guarantee_after ? Math.max(0, 1 - remainingGuarantee / snapshot.config.guarantee_after) * 100 : 0}%` }"><span>{{ snapshot.config.guarantee_after ? remainingGuarantee : '—' }}</span></div><div><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.guarantee') }}</h2><p class="mt-1 text-xs leading-5 text-gray-500">{{ snapshot.config.guarantee_after ? t('luckyDraw.guaranteeShort', { count: remainingGuarantee }) : t('luckyDraw.noGuarantee') }}</p></div></div><p class="mt-4 text-xs leading-5 text-gray-500">{{ t('luckyDraw.guaranteeRule', { count: snapshot.config.guarantee_after, remaining: remainingGuarantee }) }}</p></section>
            <section class="lucky-panel p-5"><div class="flex items-center justify-between"><h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.chances') }}</h2><span class="text-3xl font-bold text-amber-600">{{ snapshot.chances }}</span></div><p class="mt-2 text-xs leading-5 text-gray-500">{{ t('luckyDraw.recharged', { amount: snapshot.total_recharged.toFixed(2), used: snapshot.draws_used }) }}</p><p v-if="snapshot.manual_chances" class="mt-1 text-xs text-gray-500">{{ t('luckyDraw.manualReceived', { count: snapshot.manual_chances }) }}</p><p class="mt-2 text-[11px] leading-5 text-gray-400">{{ t('luckyDraw.startsAt', { time: new Date(snapshot.config.recharge_starts_at).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' }) + ' (UTC+8)' }) }}</p><router-link to="/purchase" class="mt-3 inline-block text-xs font-semibold text-amber-700 hover:underline dark:text-amber-400">{{ t('luckyDraw.recharge') }} →</router-link></section>
            <section class="lucky-panel p-5"><h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.prizes') }}</h2><div class="mt-4 grid grid-cols-2 gap-x-3 gap-y-4"><div v-for="(prize, i) in snapshot.config.prizes" :key="i"><p class="break-words text-sm font-bold" :class="rarityColor(prize.amount)">{{ prize.amount ? '¥' + prize.amount : t('luckyDraw.noPrize') }}</p><p class="mt-1 text-[11px] text-gray-400">{{ (prize.weight / 100).toFixed(2) }}% · {{ t(`luckyDraw.rarity.${luckyDrawRarity(prize.amount)}`) }}</p></div></div></section>
            <section class="lucky-panel p-5"><h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.recentRecords') }}</h2><ul v-if="snapshot.history.length" class="mt-3 max-h-64 divide-y divide-amber-100 overflow-y-auto dark:divide-dark-700"><li v-for="record in snapshot.history.slice(0, 8)" :key="record.id" class="flex items-center justify-between gap-2 py-3 text-xs"><div class="min-w-0"><p class="truncate font-medium text-gray-800 dark:text-gray-200">{{ record.prize_name }}</p><time class="mt-1 block text-[10px] text-gray-400">{{ new Date(record.created_at).toLocaleString(locale) }}</time></div><span class="shrink-0" :class="rarityColor(record.prize_amount)">{{ record.prize_amount ? '+¥' + record.prize_amount.toFixed(2) : '—' }}</span></li></ul><p v-else class="mt-3 text-xs leading-6 text-gray-500">{{ t('luckyDraw.empty') }}</p></section>
          </aside>
        </div>
        <details class="lucky-panel p-5"><summary class="cursor-pointer text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('luckyDraw.rulesTitle') }}</summary><p class="mt-3 text-xs leading-7 text-gray-500">{{ t('luckyDraw.rules') }}</p></details>
        <LuckyDrawResultModal v-if="showResult && results.length" :records="results" :demo="demo" :skip-animation="skipAnimation" :can-again="!busy && !loading && (demo || snapshot.config.enabled && snapshot.chances >= results.length)" @close="showResult = false" @again="again" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LuckyDrawStage from '@/components/lottery/LuckyDrawStage.vue'
import LuckyDrawReveal from '@/components/lottery/LuckyDrawReveal.vue'
import LuckyDrawResultModal from '@/components/lottery/LuckyDrawResultModal.vue'
import { luckyDrawRarity, luckyDrawRequestID } from '@/utils/luckyDraw'
import { luckyDrawAPI, type LuckyDrawSnapshot, type LuckyDrawRecord, type LuckyDrawRequest, type LuckyDrawPrize } from '@/api/luckyDraw'
import { useAuthStore } from '@/stores/auth'
const { t, locale } = useI18n()
const auth = useAuthStore()
const snapshot = ref<LuckyDrawSnapshot | null>(null)
const highestPrize = computed(() => Math.max(0, ...(snapshot.value?.config.prizes.map(p => p.amount) ?? [])))
const rarityColor = (amount: number) => ({ legendary: 'text-amber-500', epic: 'text-purple-500', rare: 'text-blue-500', common: 'text-emerald-500', none: 'text-gray-500' })[luckyDrawRarity(amount)]
const mode = ref<'cards' | 'wheel' | 'roller'>('cards')
const modes = [{ value: 'cards', label: 'luckyDraw.cards' }, { value: 'wheel', label: 'luckyDraw.wheel' }, { value: 'roller', label: 'luckyDraw.roller' }] as const
const result = ref<LuckyDrawRecord | null>(null)
const results = ref<LuckyDrawRecord[]>([])
const showResult = ref(false)
const batchTotal = computed(() => results.value.reduce((sum, r) => sum + Math.round(r.prize_amount * 100), 0) / 100)
const drawCounts = [1, 5, 10] as const
const drawCount = ref<1 | 5 | 10>(1)
const demo = ref(false)
const skipAnimation = ref(false)
const remainingGuarantee = computed(() => Math.max(1, (snapshot.value?.config.guarantee_after ?? 10) - (snapshot.value?.misses ?? 0)))
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
    return value && typeof value.request_id === 'string' && Number.isInteger(value.config_version) && (value.count === undefined || [1, 5, 10].includes(value.count)) ? value : null
  } catch { return null }
}
function remember(value: LuckyDrawRequest | null) {
  pending.value = value
  try { if (value) localStorage.setItem(storageKey, JSON.stringify(value)); else localStorage.removeItem(storageKey) } catch { /* Retry still works during this visit. */ }
}
const canDraw = computed(() => !loading.value && (demo.value || !!pending.value || !!snapshot.value?.config.enabled && snapshot.value.chances >= drawCount.value))
function clearResults() { showResult.value = false; result.value = null; results.value = []; if (snapshot.value) stagePrizes.value = snapshot.value.config.prizes }
function again() { const count = results.value.length; drawCount.value = count === 10 ? 10 : count === 5 ? 5 : 1; showResult.value = false; void draw(0) }
function changeMode(value: typeof mode.value) {
  mode.value = value
  clearResults()
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
  busy.value = true; error.value = ''; clearResults(); selected.value = card
  stagePrizes.value = snapshot.value.config.prizes
  try {
    let records: LuckyDrawRecord[]
    if (demo.value) {
      let misses = snapshot.value.misses
      records = Array.from({ length: drawCount.value }, (_, i) => {
        const guaranteed = snapshot.value!.config.guarantee_after > 0 && misses >= snapshot.value!.config.guarantee_after - 1
        const pool = stagePrizes.value.map((prize, index) => ({ ...prize, index })).filter(p => !guaranteed || p.amount > 0)
        let target = Math.random() * pool.reduce((sum, p) => sum + p.weight, 0)
        const prize = pool.find(p => { target -= p.weight; return target < 0 }) ?? pool[0]!
        misses = prize.amount > 0 ? 0 : misses + 1
        return { id: -Date.now() - i, request_id: 'demo', config_version: snapshot.value!.config.version, prize_index: prize.index, prize_name: prize.name, prize_amount: prize.amount, guaranteed, created_at: new Date().toISOString() }
      })
    } else {
      if (!pending.value) remember({ request_id: luckyDrawRequestID(), config_version: snapshot.value.config.version, ...(drawCount.value > 1 ? { count: drawCount.value } : {}) })
      records = (pending.value!.count ?? 1) === 1 ? [await luckyDrawAPI.draw(pending.value!)] : (await luckyDrawAPI.drawBatch(pending.value!)).records
      remember(null)
    }
    const record = records.reduce((best, r) => r.prize_amount > best.prize_amount ? r : best)
    if (record.config_version !== snapshot.value.config.version) mode.value = 'cards'
    result.value = record
    results.value = records
    if (!skipAnimation.value) await new Promise(resolve => setTimeout(resolve, mode.value === 'cards' ? 600 : 1900))
    if (!demo.value) {
      await load()
      if (records.some(r => r.prize_amount > 0)) void auth.refreshUser().catch(() => {})
    }
    showResult.value = true
  } catch (err: unknown) {
    const e = err as { reason?: string; status?: number; response?: { status?: number } }
    const reason = e.reason
    if (reason === 'LUCKY_DRAW_CHANGED' || reason === 'LUCKY_DRAW_NO_CHANCES' || reason === 'LUCKY_DRAW_PAUSED' || reason === 'INVALID_LUCKY_DRAW_REQUEST' || reason === 'LUCKY_DRAW_REQUEST_CONFLICT') remember(null)
    await load()
    error.value = reason === 'LUCKY_DRAW_CHANGED' ? t('luckyDraw.changed') : reason === 'LUCKY_DRAW_NO_CHANCES' ? t('luckyDraw.noChances') : reason === 'LUCKY_DRAW_PAUSED' ? t('luckyDraw.paused') : t('luckyDraw.drawFailed')
  } finally { busy.value = false }
}
onMounted(load)
</script>

<style scoped>
.lucky-eyebrow { color: #a77b36; font-size: 11px; letter-spacing: .15em; }
.top-prize { display: flex; align-items: center; gap: 12px; margin-left: auto; color: #9b6b20; font-size: 12px; }
.top-prize strong { font-size: 28px; letter-spacing: -.04em; }
.lucky-panel { background: #fffcf7; border: 1px solid #e9ddc5; border-radius: 18px; }
.lucky-arena { position: relative; overflow: hidden; border: 1px solid #e9ddc5; border-radius: 22px; padding: 20px; background: radial-gradient(ellipse at 50% 40%, #ecddb84d, transparent 65%), linear-gradient(135deg,#f8f3e8,#f5f2e9); }
.arena-toolbar { display: flex; align-items: center; justify-content: center; gap: 16px; flex-wrap: wrap; padding-bottom: 16px; border-bottom: 1px solid #b68c3f20; }
.mode-button,.count-button { border-radius: 8px; padding: 8px 14px; color: #8d7954; font-size: 12px; transition: background .2s; }
.mode-button.active,.count-button.active { color: #8c5b13; background: #dfc79860; box-shadow: inset 0 0 0 1px #b8893930; }
.mode-button:disabled,.count-button:disabled { cursor: default; }
.demo-switch { display: flex; align-items: center; gap: 7px; color: #9d752f; font-size: 12px; cursor: pointer; }
.demo-notice { max-width: 500px; margin: 16px auto 0; border: 1px solid #d6bb80; border-radius: 10px; padding: 10px 14px; color: #90723d; font-size: 11px; line-height: 1.7; background: #e5d5b433; }
.arena-content { min-height: 475px; padding-top: 32px; padding-bottom: 24px; }
.results-area { max-width: 660px; margin: 0 auto; }
.draw-controls { display: flex; align-items: center; justify-content: center; gap: 12px; flex-wrap: wrap; max-width: 650px; margin: 0 auto; border-radius: 12px; padding: 10px; border: 1px solid #e5d9c1; background: #fffdf8b3; color: #8a744a; }
.count-button { padding: 7px 12px; }
.draw-button { border-radius: 9px; padding: 9px 15px; background: linear-gradient(135deg,#b68b3c,#8f601d); color: #fff8e7; font-size: 12px; font-weight: 600; box-shadow: 0 3px 8px #8f601d20; }
.draw-button:disabled { opacity: .45; }
.guarantee-ring { --progress: 0%; display: grid; place-items: center; flex-shrink: 0; width: 62px; height: 62px; border-radius: 50%; background: conic-gradient(#b88939 var(--progress),#e8dfca 0); padding: 5px; }
.guarantee-ring span { display: grid; place-items: center; width: 100%; height: 100%; border-radius: 50%; background: #fffcf7; color: #996a20; font-size: 22px; font-weight: 700; }
.skip-animation :deep(*) { animation: none !important; transition: none !important; }
:global(.dark) .lucky-panel { background: #211f1b; border-color: #50452f; }
:global(.dark) .lucky-arena { background: radial-gradient(ellipse at 50% 40%,#8b6a2020,transparent 70%),#211f1b; border-color: #50452f; }
:global(.dark) .draw-controls { background: #2c271f; border-color: #50452f; color: #bfa577; }
:global(.dark) .mode-button { color: #bfa577; }
:global(.dark) .guarantee-ring span { background: #211f1b; color: #e0bb70; }
@media (max-width: 640px) { .lucky-arena { padding: 14px; } .arena-content { min-height: 420px; padding-top: 24px; } .draw-controls { gap: 8px; } }
@media (prefers-reduced-motion: reduce) { * { transition: none !important; animation: none !important; } }
</style>
