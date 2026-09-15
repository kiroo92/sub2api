<template>
  <section id="redeem" class="scroll-mt-20 space-y-4 border-t border-gray-200 pt-5 dark:border-dark-700" aria-labelledby="redeem-heading">
    <h2 id="redeem-heading" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('redeem.redeemCodeLabel') }}</h2>
    <form class="flex flex-col gap-3 sm:flex-row" @submit.prevent="handleRedeem">
      <label for="redeem-code" class="sr-only">{{ t('redeem.redeemCodeLabel') }}</label>
      <input id="redeem-code" v-model="redeemCode" type="text" required autocomplete="off" :placeholder="t('redeem.redeemCodePlaceholder')" :disabled="submitting" class="input min-w-0 flex-1" />
      <button type="submit" :disabled="!redeemCode.trim() || submitting || refreshing" class="btn btn-primary inline-flex shrink-0 items-center justify-center gap-2">
        <Icon :name="submitting ? 'refresh' : 'gift'" size="sm" :class="{ 'animate-spin': submitting }" />
        {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
      </button>
    </form>
    <div v-if="redeemResult" role="status" class="flex gap-3 rounded-md bg-emerald-50 p-3 text-sm text-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300">
      <Icon name="checkCircle" size="md" class="shrink-0" />
      <div class="min-w-0 space-y-1 break-words">
        <p class="font-medium">{{ t('redeem.redeemSuccess') }}</p>
        <p>{{ redeemResult.message }}</p>
        <p v-if="redeemResult.type === 'balance'">{{ t('redeem.added') }}: {{ formatCurrency(redeemResult.value) }}</p>
        <p v-else-if="redeemResult.type === 'concurrency'">{{ t('redeem.added') }}: {{ redeemResult.value }} {{ t('redeem.concurrentRequests') }}</p>
        <p v-else-if="redeemResult.type === 'subscription'">
          {{ t('redeem.subscriptionAssigned') }}<span v-if="redeemResult.group_name"> · {{ redeemResult.group_name }}</span>
          <span v-if="redeemResult.validity_days"> ({{ t('redeem.subscriptionDays', { days: redeemResult.validity_days }) }})</span>
        </p>
        <p v-if="redeemResult.new_balance !== undefined">{{ t('redeem.newBalance') }}: {{ formatCurrency(redeemResult.new_balance) }}</p>
        <p v-if="redeemResult.new_concurrency !== undefined">{{ t('redeem.newConcurrency') }}: {{ redeemResult.new_concurrency }} {{ t('redeem.requests') }}</p>
      </div>
    </div>
    <p v-if="errorMessage" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>
    <p v-if="refreshFailed" role="alert" class="text-sm text-amber-700 dark:text-amber-300">
      {{ t('redeem.accountRefreshFailed') }}
      <button type="button" class="underline" :disabled="refreshing || submitting" @click="refreshAccount">{{ t('common.refresh') }}</button>
    </p>
    <details class="text-sm">
      <summary class="cursor-pointer text-gray-500 dark:text-gray-400">{{ t('redeem.recentActivity') }}</summary>
      <p v-if="historyFailed" role="alert" class="mt-3 text-red-600 dark:text-red-400">{{ t('redeem.historyLoadFailed') }} <button type="button" :disabled="loadingHistory" class="underline" @click="fetchHistory">{{ t('common.refresh') }}</button></p>
      <p v-if="loadingHistory && !history.length" class="py-4 text-gray-500" role="status">{{ t('common.loading') }}</p>
      <ul v-else-if="history.length" class="mt-3 divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="item in history" :key="item.id" class="flex flex-wrap items-start justify-between gap-2 py-3">
          <div class="min-w-0">
            <p class="font-medium text-gray-900 dark:text-white">{{ getHistoryItemTitle(item) }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(item.used_at) }}</p>
          </div>
          <div class="min-w-0 text-right">
            <p class="break-words font-medium" :class="item.value < 0 ? 'text-red-600 dark:text-red-400' : 'text-emerald-700 dark:text-emerald-400'">{{ formatHistoryValue(item) }}</p>
            <p class="mt-1 text-xs text-gray-400">{{ isAdminAdjustment(item.type) ? t('redeem.adminAdjustment') : item.code.slice(0, 8) + '...' }}</p>
            <p v-if="item.notes" class="mt-1 max-w-xs break-words text-xs text-gray-500 dark:text-gray-400">{{ item.notes }}</p>
          </div>
        </li>
      </ul>
      <p v-else-if="!historyFailed" class="py-4 text-gray-500 dark:text-gray-400">{{ t('redeem.historyWillAppear') }}</p>
    </details>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { redeemAPI, type RedeemHistoryItem } from '@/api/redeem'
import Icon from '@/components/icons/Icon.vue'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()
const emit = defineEmits<{ redeemed: []; refresh: [accountRefreshed: boolean] }>()
const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<(Awaited<ReturnType<typeof redeemAPI.redeem>> & { group_name?: string; validity_days?: number }) | null>(null)
const errorMessage = ref('')
const refreshing = ref(false)
const refreshFailed = ref(false)
const history = ref<RedeemHistoryItem[]>([])
const loadingHistory = ref(false)
const historyFailed = ref(false)
let historyRefreshPending = false

const isBalanceType = (type: string) => type === 'balance' || type === 'admin_balance'
const isAdminAdjustment = (type: string) => type === 'admin_balance' || type === 'admin_concurrency'
function getHistoryItemTitle(item: RedeemHistoryItem) {
  if (item.type === 'balance') return t('redeem.balanceAddedRedeem')
  if (item.type === 'admin_balance') return t(item.value >= 0 ? 'redeem.balanceAddedAdmin' : 'redeem.balanceDeductedAdmin')
  if (item.type === 'concurrency') return t('redeem.concurrencyAddedRedeem')
  if (item.type === 'admin_concurrency') return t(item.value >= 0 ? 'redeem.concurrencyAddedAdmin' : 'redeem.concurrencyReducedAdmin')
  if (item.type === 'subscription') return t('redeem.subscriptionAssigned')
  return t('common.unknown')
}
function formatHistoryValue(item: RedeemHistoryItem) {
  if (isBalanceType(item.type)) return `${item.value >= 0 ? '+' : ''}${formatCurrency(item.value)}`
  if (item.type === 'subscription') {
    const days = item.validity_days || Math.round(item.value)
    return `${days}${t('redeem.days')}${item.group?.name ? ' - ' + item.group.name : ''}`
  }
  return `${item.value >= 0 ? '+' : ''}${item.value} ${t('redeem.requests')}`
}

async function fetchHistory() {
  if (loadingHistory.value) { historyRefreshPending = true; return }
  loadingHistory.value = true
  try {
    history.value = await redeemAPI.getHistory()
    historyFailed.value = false
  } catch {
    historyFailed.value = true
  } finally {
    loadingHistory.value = false
    if (historyRefreshPending) { historyRefreshPending = false; void fetchHistory() }
  }
}

async function refreshAccount() {
  if (refreshing.value) return
  refreshing.value = true
  const results = await Promise.allSettled([
    authStore.refreshUser(),
    ...(redeemResult.value?.type === 'subscription' ? [subscriptionStore.fetchActiveSubscriptions(true)] : [])
  ])
  refreshFailed.value = results.some(result => result.status === 'rejected')
  refreshing.value = false
  emit('refresh', results[0].status === 'fulfilled')
}

async function handleRedeem() {
  if (submitting.value || refreshing.value) return
  if (!redeemCode.value.trim()) { appStore.showError(t('redeem.pleaseEnterCode')); return }
  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null
  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())
    // Confirmed redemption is final; a failed refresh must never invite another submission.
    redeemResult.value = result
    redeemCode.value = ''
    emit('redeemed')
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
    await Promise.all([refreshAccount(), fetchHistory()])
  } catch (error) {
    errorMessage.value = extractApiErrorMessage(error, t('redeem.failedToRedeem'))
    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}
onMounted(fetchHistory)
</script>
