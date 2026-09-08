<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div v-if="error" role="alert" class="flex items-center justify-between gap-4 rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
        <span>{{ error }}</span><button class="btn btn-secondary shrink-0" :disabled="loading" @click="load()">{{ t('lottery.refresh') }}</button>
      </div>
      <div v-if="!snapshot && loading" role="status" class="card p-12 text-center text-gray-500">{{ t('lottery.loading') }}</div>

      <template v-if="snapshot">
        <div class="grid items-stretch gap-4 lg:grid-cols-5">
          <section class="card overflow-hidden lg:col-span-3" aria-labelledby="lottery-title">
            <div class="flex items-start justify-between gap-4 border-b border-gray-100 p-6 dark:border-dark-700">
              <div><h1 id="lottery-title" class="text-xl font-bold text-gray-900 dark:text-gray-100">{{ t('lottery.title') }}</h1><p class="mt-2 max-w-md text-sm leading-relaxed text-gray-500 dark:text-gray-400">{{ t('lottery.description') }}</p></div>
              <div class="flex shrink-0 items-center gap-2">
                <button class="btn btn-secondary" :disabled="loading || joining" :aria-label="t('lottery.refresh')" @click="load()"><Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" /><span class="hidden sm:inline">{{ t('lottery.refresh') }}</span></button>
                <span class="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400" aria-hidden="true"><Icon name="gift" size="md" /></span>
              </div>
            </div>
            <div class="space-y-6 p-6">
              <dl class="grid grid-cols-3 gap-3">
                <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><dt class="text-xs text-gray-500">{{ t('lottery.prizeAmount') }}</dt><dd class="mt-2 text-xl font-bold tabular-nums text-gray-900 dark:text-gray-100 sm:text-2xl">${{ rules.prize_amount.toFixed(2) }}</dd></div>
                <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><dt class="text-xs text-gray-500">{{ t('lottery.prizes') }}</dt><dd class="mt-2 text-2xl font-bold tabular-nums text-gray-900 dark:text-gray-100">{{ rules.winner_count }}</dd></div>
                <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><dt class="text-xs text-gray-500">{{ t('lottery.round') }}</dt><dd class="mt-2 text-2xl font-bold tabular-nums text-gray-900 dark:text-gray-100">{{ snapshot.current ? '#' + snapshot.current.id : '—' }}</dd></div>
              </dl>
              <div>
                <div class="mb-2 flex justify-between text-sm text-gray-600 dark:text-gray-400"><span>{{ t('lottery.progress') }}</span><span class="tabular-nums">{{ snapshot.current?.participant_count || 0 }} / {{ rules.participant_target }}</span></div>
                <div role="progressbar" :aria-label="t('lottery.progress')" :aria-valuenow="snapshot.current?.participant_count || 0" :aria-valuemax="rules.participant_target" :aria-valuemin="0" class="h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full bg-primary-500 transition-all motion-reduce:transition-none" :style="{ width: progress + '%' }"></div></div>
              </div>
              <div class="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
                <div aria-live="polite">
                  <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ participationLabel }}</p>
                  <p class="mt-1 text-xs leading-relaxed text-gray-500">{{ t('lottery.eligibilityHint', { amount: rules.min_recharge.toFixed(2) }) }}</p>
                  <p v-if="!snapshot.eligible && snapshot.config.enabled && !snapshot.joined" class="mt-1 text-xs text-gray-500">{{ t('lottery.yourRecharge', { amount: snapshot.total_recharged.toFixed(2) }) }}</p>
                </div>
                <button data-testid="lottery-join" class="btn btn-primary min-w-28 shrink-0" :disabled="!canJoin || !captchaReady || joining || loading" @click="join"><Icon name="gift" size="sm" />{{ joining ? t('lottery.joining') : snapshot.joined ? t('lottery.joined') : t('lottery.join') }}</button>
              </div>
              <div class="lottery-captcha">
      <CaptchaChallenge v-if="captchaReady" ref="captchaRef" :turnstile-enabled="false" turnstile-site-key=""
        :tencent-enabled="captchaSettings?.tencent_captcha_enabled === true" :tencent-app-id="captchaSettings?.tencent_captcha_app_id || ''" :tencent-region="captchaSettings?.tencent_captcha_region"
        :aliyun-enabled="captchaSettings?.aliyun_captcha_enabled === true" :aliyun-scene-id="captchaSettings?.aliyun_captcha_scene_id" :aliyun-prefix="captchaSettings?.aliyun_captcha_prefix" :aliyun-region="captchaSettings?.aliyun_captcha_region"
        @error="error = t('lottery.captchaFailed')" />
              </div>
              <p v-if="!captchaReady" class="text-sm text-amber-600 dark:text-amber-400">{{ t('lottery.captchaUnavailable') }}</p>
              <p v-if="feedback" role="status" class="text-sm text-emerald-600 dark:text-emerald-400">{{ feedback }}</p>
              <p class="text-xs text-gray-400">{{ t('lottery.freeEntry') }}</p>
            </div>
          </section>
          <section class="card flex max-h-[460px] flex-col overflow-hidden lg:col-span-2" aria-labelledby="lottery-winners-title">
            <h2 id="lottery-winners-title" class="border-b border-gray-100 px-6 py-5 text-lg font-semibold text-gray-900 dark:border-dark-700 dark:text-gray-100">{{ t('lottery.recentWinners') }}</h2>
            <ul v-if="snapshot.recent_winners.length" class="space-y-2 overflow-y-auto p-4">
              <li v-for="(win, index) in snapshot.recent_winners" :key="win.round_id + '-' + index" class="flex items-center justify-between gap-3 rounded-lg bg-gray-50 px-3 py-2.5 text-sm dark:bg-dark-800"><div class="min-w-0"><p class="truncate text-gray-800 dark:text-gray-200">{{ win.user_label }}</p><p class="mt-0.5 text-xs text-gray-500">{{ t('lottery.roundNumber', { number: win.round_id }) }}</p></div><span class="shrink-0 font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">+${{ win.prize_amount.toFixed(2) }}</span></li>
            </ul>
            <div v-else class="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-14 text-gray-400"><Icon name="gift" size="xl" /><p class="text-sm">{{ t('lottery.noWinners') }}</p></div>
          </section>
        </div>
        <section class="card overflow-hidden" aria-labelledby="lottery-my-wins">
          <h2 id="lottery-my-wins" class="flex items-center gap-2 border-b border-gray-100 px-6 py-5 text-lg font-semibold text-gray-900 dark:border-dark-700 dark:text-gray-100"><Icon name="gift" size="sm" class="text-amber-500" />{{ t('lottery.myWins') }}</h2>
          <ul v-if="snapshot.my_wins.length" class="divide-y divide-gray-100 dark:divide-dark-700"><li v-for="win in snapshot.my_wins" :key="win.round_id" class="flex flex-wrap items-center justify-between gap-3 px-6 py-4 text-sm"><div><span class="font-medium text-gray-800 dark:text-gray-200">{{ t('lottery.roundNumber', { number: win.round_id }) }}</span><time class="ml-3 text-xs text-gray-500" :datetime="win.awarded_at">{{ formatDate(win.awarded_at) }}</time></div><span class="font-semibold text-emerald-600 dark:text-emerald-400">+${{ win.prize_amount.toFixed(2) }} <span class="ml-2 text-xs font-normal text-gray-500">{{ t('lottery.credited') }}</span></span></li></ul>
          <p v-else class="px-6 py-10 text-center text-sm text-gray-500">{{ t('lottery.noMyWins') }}</p>
        </section>
        <LotteryRoundsTable :rounds="snapshot.recent_rounds" :enabled="snapshot.config.enabled" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CaptchaChallenge from '@/components/CaptchaChallenge.vue'
import { getPublicSettings } from '@/api/auth'
import type { PublicSettings } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LotteryRoundsTable from '@/components/lottery/LotteryRoundsTable.vue'
import { lotteryAPI, type LotterySnapshot } from '@/api/lottery'
import { useAuthStore } from '@/stores/auth'
const { t, locale } = useI18n()
const auth = useAuthStore()
const captchaRef = ref<InstanceType<typeof CaptchaChallenge> | null>(null)
const captchaSettings = ref<PublicSettings | null>(null)
const captchaReady = computed(() => {
  const c = captchaSettings.value
  if (!c || c.turnstile_enabled || (c.tencent_captcha_enabled && c.aliyun_captcha_enabled)) return false
  return !!((c.tencent_captcha_enabled && c.tencent_captcha_app_id) || (c.aliyun_captcha_enabled && c.aliyun_captcha_scene_id && c.aliyun_captcha_prefix))
})
const snapshot = ref<LotterySnapshot | null>(null)
const loading = ref(false)
const joining = ref(false)
const error = ref('')
const feedback = ref('')
const rules = computed(() => snapshot.value!.current ?? snapshot.value!.config)
const progress = computed(() => Math.min(100, (snapshot.value?.current?.participant_count || 0) / rules.value.participant_target * 100))
const canJoin = computed(() => !!snapshot.value?.config.enabled && !!snapshot.value.current && snapshot.value.eligible && !snapshot.value.joined)
const participationLabel = computed(() => !snapshot.value?.config.enabled ? t('lottery.paused') : snapshot.value.joined ? t('lottery.participating') : snapshot.value.eligible ? t('lottery.ready') : t('lottery.notEligible'))
const formatDate = (value: string) => new Date(value).toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
let timer: ReturnType<typeof setInterval> | undefined
let disposed = false
async function load(silent = false) {
  if (loading.value) return
  loading.value = true
  try {
    if (!silent) captchaSettings.value = await getPublicSettings()
    const data = await lotteryAPI.get()
    if (disposed) return
    const latestWin = snapshot.value?.my_wins[0]?.round_id
    snapshot.value = data
    error.value = ''
    if (data.my_wins[0]?.round_id && data.my_wins[0].round_id !== latestWin) void auth.refreshUser().catch(() => {})
  } catch { if (!silent && !disposed) error.value = t('lottery.loadFailed') }
  finally { loading.value = false }
}
async function join() {
  if (!canJoin.value || !captchaReady.value || joining.value || !snapshot.value?.current) return
  const roundID = snapshot.value.current.id
  joining.value = true; error.value = ''; feedback.value = ''
  try {
    captchaRef.value?.reset()
    const challenge = await captchaRef.value?.verifyAction()
    if (!challenge || disposed) return
    const proof = captchaSettings.value?.tencent_captcha_enabled
      ? { tencent_captcha_ticket: challenge.token, tencent_captcha_randstr: challenge.randstr }
      : { turnstile_token: challenge.token }
    const result = await lotteryAPI.join(roundID, proof)
    feedback.value = result.drawn ? t('lottery.drawComplete') : t('lottery.joinSuccess')
    await load()
  } catch (err: unknown) {
    await load()
    const reason = (err as { reason?: string })?.reason
    error.value = reason?.includes('CAPTCHA') ? t('lottery.captchaFailed') : reason === 'LOTTERY_ROUND_CHANGED' ? t('lottery.roundChanged') : reason === 'LOTTERY_NOT_ELIGIBLE' ? t('lottery.notEligible') : reason === 'LOTTERY_PAUSED' ? t('lottery.paused') : t('lottery.joinFailed')
  } finally { captchaRef.value?.reset(); joining.value = false }
}
onMounted(() => { void load(); timer = setInterval(() => { if (!document.hidden && !joining.value) void load(true) }, 15000) })
onUnmounted(() => { disposed = true; if (timer) clearInterval(timer) })
</script>

<style scoped>
/* The participation button opens the provider popup through verifyAction(). */
.lottery-captcha :deep(.aliyun-captcha-button) { display: none; }
</style>
