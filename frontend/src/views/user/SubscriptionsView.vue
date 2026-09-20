<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div><p class="mb-1 text-xs font-semibold uppercase tracking-widest text-primary-600">{{ t('userSubscriptions.overview') }}</p><h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('userSubscriptions.title') }}</h1></div>
        <p class="text-sm text-gray-500">{{ t('userSubscriptions.counts', { active: availableCount, frozen: frozenCount }) }}</p>
      </header>
      <section v-if="ordered.length" class="rounded-xl border border-primary-200 bg-primary-50/60 px-4 py-3 dark:border-primary-800 dark:bg-primary-900/10">
        <p class="text-sm font-semibold text-primary-800 dark:text-primary-200">{{ t('userSubscriptions.orderTitle') }}</p>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ t('userSubscriptions.orderHint') }}</p>
        <p class="mt-1 text-xs text-gray-500" aria-live="polite">{{ savingOrder ? t('userSubscriptions.savingOrder') : orderMessage || t('userSubscriptions.orderAutoSave') }}</p>
      </section>
      <div v-if="loading" class="py-12 text-center text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="!subscriptions.length" class="card p-10 text-center">
        <h3 class="font-semibold">{{ t('userSubscriptions.noActiveSubscriptions') }}</h3><p class="mt-2 text-sm text-gray-500">{{ t('userSubscriptions.noActiveSubscriptionsDesc') }}</p>
      </div>
      <template v-for="(subscription, index) in displayed" :key="subscription.id">
        <h2 v-if="index === current.length && history.length" class="pt-3 text-sm font-semibold text-gray-500">{{ t('userSubscriptions.history') }}</h2>
        <article class="overflow-hidden rounded-2xl border bg-white shadow-sm dark:bg-dark-800" :class="subscription.frozen_at ? 'border-sky-200 dark:border-sky-900' : 'border-gray-200 dark:border-dark-600'" :data-subscription-id="subscription.id">
          <div class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="flex min-w-0 items-start gap-3">
              <span v-if="activeIndex(subscription.id) >= 0" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-100 text-lg font-semibold tabular-nums text-gray-500 dark:bg-dark-700" :aria-label="t('userSubscriptions.orderNumber', { number: activeIndex(subscription.id) + 1 })">{{ String(activeIndex(subscription.id) + 1).padStart(2, '0') }}</span>
              <div class="min-w-0">
                <h3 class="break-words font-semibold text-gray-900 dark:text-white">{{ subscription.group?.name || `Group #${subscription.group_id}` }}</h3>
                <div class="mt-1.5 flex flex-wrap items-center gap-2 text-xs">
                  <span class="rounded-md border px-1.5 py-0.5" :class="platformBadgeClass(subscription.group?.platform || '')">{{ platformLabel(subscription.group?.platform || '') }}</span>
                  <span class="text-gray-400">#{{ subscription.id }}</span>
                  <span class="rounded-full px-2 py-0.5" :class="subscription.frozen_at ? 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300' : subscription.status === 'active' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700'">{{ subscription.frozen_at ? t('userSubscriptions.frozen') : t(`userSubscriptions.status.${subscription.status}`) }}</span>
                  <span v-if="subscription.frozen_at && subscription.status === 'suspended'" class="text-red-600">{{ t('userSubscriptions.status.suspended') }}</span>
                </div>
              </div>
            </div>
            <div class="text-right">
              <p class="text-xs text-gray-400">{{ t(subscription.frozen_at ? 'userSubscriptions.preservedTime' : 'userSubscriptions.remainingTime') }}</p>
              <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ duration(remaining(subscription)) }}</p>
              <p class="mt-1 text-xs text-gray-400">{{ subscription.frozen_at ? t('userSubscriptions.frozenHint') : subscription.expires_at ? formatDateTimeToMinute(subscription.expires_at) : t('userSubscriptions.noExpiration') }}</p>
            </div>
          </div>
          <div class="grid gap-4 px-5 py-4 sm:grid-cols-3">
            <div v-for="window in windows" :key="window.key" class="min-w-0">
              <div class="mb-2 flex items-center justify-between gap-2 text-xs"><span class="font-medium text-gray-500">{{ t(`userSubscriptions.${window.key}`) }}</span><span class="tabular-nums text-gray-700 dark:text-gray-300">{{ money(subscription[window.used]) }} / {{ subscription.group?.[window.limit] ? money(subscription.group[window.limit]!) : t('userSubscriptions.unlimited') }}</span></div>
              <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full rounded-full transition-all" :class="subscription.frozen_at ? 'bg-sky-400' : percent(subscription, window) >= 90 ? 'bg-amber-500' : 'bg-primary-500'" :style="{ width: percent(subscription, window) + '%' }" /></div>
              <p class="mt-2 text-xs text-gray-400">{{ resetLabel(subscription, window) }}</p>
            </div>
          </div>
          <footer class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 bg-gray-50/60 px-5 py-3 dark:border-dark-700 dark:bg-dark-900/30">
            <p class="text-xs text-gray-500">{{ t('payment.planCard.rate') }} ×{{ subscription.group?.rate_multiplier ?? 1 }}<span v-if="subscription.group && hasPeakRate(subscription.group)"> · {{ formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)) }}</span></p>
            <div class="flex flex-wrap items-center gap-2">
              <template v-if="activeIndex(subscription.id) >= 0">
                <button class="btn btn-secondary btn-sm gap-1.5" :disabled="busy || activeIndex(subscription.id) === 0" :title="t('userSubscriptions.moveUp')" @click="moveSubscription(subscription.id, -1)"><Icon name="chevronUp" size="sm" />{{ t('userSubscriptions.moveUp') }}</button>
                <button class="btn btn-secondary btn-sm gap-1.5" :disabled="busy || activeIndex(subscription.id) === ordered.length - 1" :title="t('userSubscriptions.moveDown')" @click="moveSubscription(subscription.id, 1)"><Icon name="chevronDown" size="sm" />{{ t('userSubscriptions.moveDown') }}</button>
              </template>
              <button v-if="subscription.frozen_at && subscription.status !== 'revoked'" class="btn btn-primary btn-sm" :disabled="busy" @click="pending = subscription">{{ t('userSubscriptions.unfreeze') }}</button>
              <button v-else-if="freezeEnabled && isHeld(subscription)" class="btn btn-secondary btn-sm" :disabled="busy" @click="pending = subscription">{{ t('userSubscriptions.freeze') }}</button>
            </div>
          </footer>
        </article>
      </template>
      <BaseDialog :show="!!pending" :title="t(pending?.frozen_at ? 'userSubscriptions.unfreeze' : 'userSubscriptions.freeze')" :show-close-button="!changing" :close-on-escape="!changing" @close="!changing && (pending = null)">
        <p class="text-sm leading-6 text-gray-600 dark:text-gray-300">{{ t(pending?.frozen_at ? 'userSubscriptions.unfreezeConfirm' : 'userSubscriptions.freezeConfirm') }}</p>
        <p v-if="actionError" role="alert" class="mt-3 text-sm text-red-600">{{ actionError }}</p>
        <template #footer><button class="btn btn-secondary" :disabled="changing" @click="pending = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="changing" @click="changeFreeze">{{ t(changing ? 'common.processing' : 'common.confirm') }}</button></template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformBadgeClass, platformLabel } from '@/utils/platformColors'

const { t } = useI18n()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()
const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)
const savingOrder = ref(false)
const changing = ref(false)
const pending = ref<UserSubscription | null>(null)
const actionError = ref('')
const orderMessage = ref('')
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined
const busy = computed(() => savingOrder.value || changing.value)
const freezeEnabled = computed(() => appStore.cachedPublicSettings?.subscription_freeze_enabled === true)
const isHeld = (sub: UserSubscription) => sub.status === 'active' && (!!sub.frozen_at || !sub.expires_at || Date.parse(sub.expires_at) > now.value)
const ordered = computed(() => subscriptions.value.filter(isHeld))
const current = computed(() => subscriptions.value.filter(sub => isHeld(sub) || (!!sub.frozen_at && sub.status !== 'revoked')))
const history = computed(() => subscriptions.value.filter(sub => !current.value.includes(sub)))
const displayed = computed(() => [...current.value, ...history.value])
const availableCount = computed(() => ordered.value.filter(sub => !sub.frozen_at).length)
const frozenCount = computed(() => current.value.filter(sub => !!sub.frozen_at).length)
const windows = [
  { key: 'daily', used: 'daily_usage_usd', limit: 'daily_limit_usd', reset: 'daily_resets_at', start: 'daily_window_start', hours: 24 },
  { key: 'weekly', used: 'weekly_usage_usd', limit: 'weekly_limit_usd', reset: 'weekly_resets_at', start: 'weekly_window_start', hours: 168 },
  { key: 'monthly', used: 'monthly_usage_usd', limit: 'monthly_limit_usd', reset: 'monthly_resets_at', start: 'monthly_window_start', hours: 720 }
] as const
type QuotaWindow = typeof windows[number]
function activeIndex(id: number) { return ordered.value.findIndex(sub => sub.id === id) }
function money(value: number) { return '$' + (value || 0).toFixed(2) }
function percent(sub: UserSubscription, window: QuotaWindow) { const limit = sub.group?.[window.limit]; return limit ? Math.min(100, Math.max(0, sub[window.used] / limit * 100)) : 0 }
function referenceTime(sub: UserSubscription) { return sub.frozen_at ? Date.parse(sub.frozen_at) : now.value }
function remaining(sub: UserSubscription) { return sub.expires_at ? Math.max(0, (Date.parse(sub.expires_at) - referenceTime(sub)) / 1000) : null }
function duration(seconds: number | null) {
  if (seconds == null) return t('userSubscriptions.noExpiration')
  const minutes = Math.max(0, Math.ceil(seconds / 60))
  if (minutes >= 1440) return t('userSubscriptions.durationDays', { days: Math.floor(minutes / 1440), hours: Math.floor(minutes % 1440 / 60) })
  return t('userSubscriptions.durationHours', { hours: Math.floor(minutes / 60), minutes: minutes % 60 })
}
function resetLabel(sub: UserSubscription, window: QuotaWindow) {
  if (!sub.group?.[window.limit]) return t('userSubscriptions.noQuotaLimit')
  const reset = sub[window.reset]
  const legacyStart = sub[window.start]
  const at = reset ? Date.parse(reset) : legacyStart ? Date.parse(legacyStart) + window.hours * 3600000 : null
  if (at == null) return t('userSubscriptions.windowNotActive')
  const time = duration(Math.max(0, (at - referenceTime(sub)) / 1000))
  if (window.key === 'daily' && sub.is_one_time_daily_quota) return t(sub.frozen_at ? 'userSubscriptions.quotaEndsAfterThaw' : 'userSubscriptions.quotaEndsIn', { time })
  return t(sub.frozen_at ? 'userSubscriptions.resetAfterThaw' : 'userSubscriptions.resetIn', { time })
}
watch(pending, () => { actionError.value = '' })
async function loadSubscriptions() {
  try { subscriptions.value = await subscriptionsAPI.getMySubscriptions() }
  catch { appStore.showError(t('userSubscriptions.failedToLoad')) }
  finally { loading.value = false }
}
async function moveSubscription(id: number, offset: -1 | 1) {
  const from = activeIndex(id), to = from + offset
  if (busy.value || from < 0 || to < 0 || to >= ordered.value.length) return
  const previous = [...subscriptions.value]
  const next = [...ordered.value]
  ;[next[from], next[to]] = [next[to], next[from]]
  subscriptions.value = [...next, ...previous.filter(sub => !isHeld(sub))]
  savingOrder.value = true
  orderMessage.value = ''
  try { await subscriptionsAPI.reorderSubscriptions(next.map(sub => sub.id)); orderMessage.value = t('userSubscriptions.orderSaved') }
  catch { subscriptions.value = previous; orderMessage.value = t('userSubscriptions.orderRolledBack'); appStore.showError(t('userSubscriptions.failedToReorder')) }
  finally { savingOrder.value = false }
}
async function changeFreeze() {
  if (!pending.value || busy.value) return
  changing.value = true
  actionError.value = ''
  try {
    const sub = pending.value
    const changed = await (sub.frozen_at ? subscriptionsAPI.unfreezeSubscription(sub.id) : subscriptionsAPI.freezeSubscription(sub.id))
    subscriptions.value = subscriptions.value.map(item => item.id === changed.id ? changed : item)
    pending.value = null
    subscriptionStore.invalidateCache()
    await loadSubscriptions()
  } catch {
    actionError.value = t('userSubscriptions.freezeFailed')
    await appStore.fetchPublicSettings?.(true).catch(() => undefined)
  } finally { changing.value = false }
}
onMounted(() => { void loadSubscriptions(); timer = setInterval(() => { now.value = Date.now() }, 30000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>
