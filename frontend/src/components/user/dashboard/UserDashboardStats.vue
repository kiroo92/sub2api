<template>
  <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" data-testid="dashboard-summary">
    <article class="summary-tile">
      <div class="flex items-center justify-between gap-2">
        <p class="summary-label"><Icon name="dollar" size="sm" />{{ t('dashboard.todayCost') }}</p>
        <button type="button" class="-m-1 rounded p-1 text-gray-400 hover:text-gray-700 focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:text-white" :title="t('common.refresh')" :aria-label="t('common.refresh')" :disabled="refreshing" @click="$emit('refresh')">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': refreshing }" />
        </button>
      </div>
      <p class="summary-value text-gray-900 dark:text-white">{{ stats ? formatCurrency(stats.today_actual_cost) : '--' }}</p>
      <p class="summary-detail">{{ t('dashboard.standard') }} {{ stats ? formatCurrency(stats.today_cost) : '--' }}</p>
    </article>
    <article v-if="!isSimple" class="summary-tile summary-tile--balance">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p class="summary-label"><Icon name="creditCard" size="sm" />{{ t('common.availableBalance') }}</p>
        <RouterLink v-if="canRecharge" to="/purchase?tab=recharge" class="inline-flex min-h-8 shrink-0 items-center gap-1.5 rounded-md bg-emerald-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-emerald-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:ring-offset-2 dark:bg-emerald-500 dark:text-gray-950 dark:hover:bg-emerald-400">
          <Icon name="plus" size="sm" />{{ t('payment.admin.balanceOrder') }}
        </RouterLink>
      </div>
      <p class="summary-value text-emerald-700 dark:text-emerald-400">{{ balance === null ? '--' : formatCurrency(balance) }}</p>
      <p class="summary-detail">{{ t('redeem.concurrency') }}: {{ concurrency ?? '--' }}</p>
    </article>
    <article class="summary-tile">
      <p class="summary-label"><Icon name="chart" size="sm" />{{ t('dashboard.todayRequests') }}</p>
      <p class="summary-value text-sky-700 dark:text-sky-400">{{ stats ? formatNumber(stats.today_requests) : '--' }}</p>
      <p class="summary-detail">{{ t('common.total') }}: {{ stats ? formatNumber(stats.total_requests) : '--' }}</p>
    </article>
    <article class="summary-tile">
      <p class="summary-label"><Icon name="key" size="sm" />{{ t('dashboard.apiKeys') }}</p>
      <p class="summary-value text-gray-900 dark:text-white">{{ stats ? formatNumber(stats.active_api_keys) : '--' }}</p>
      <p class="summary-detail">{{ t('common.active') }} / {{ t('common.total') }}: {{ stats ? formatNumber(stats.total_api_keys) : '--' }}</p>
    </article>
  </div>
  <details v-if="!isSimple && limitedQuotas.length" class="text-sm" data-testid="platform-quotas">
    <summary class="cursor-pointer text-gray-500 dark:text-gray-400">{{ t('dashboard.platformQuota.title') }}</summary>
    <div class="mt-3 grid items-start gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <article v-for="quota in limitedQuotas" :key="quota.platform" class="min-w-0 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800">
        <h3 class="mb-3 text-sm font-medium">{{ platformLabel(quota.platform) }}</h3>
        <template v-for="window in windows" :key="window">
          <div v-if="quota[`${window}_limit_usd`] != null" class="mt-2 space-y-1 text-xs">
            <div class="flex flex-wrap justify-between gap-1">
              <span class="text-gray-500 dark:text-gray-400">{{ t(`dashboard.platformQuota.${window}`) }}</span>
              <span v-if="quota[`${window}_limit_usd`] === 0" class="text-red-600 dark:text-red-400">{{ t('dashboard.platformQuota.disabled') }}</span>
              <span v-else>{{ formatCurrency(quota[`${window}_usage_usd`]) }} / {{ formatCurrency(quota[`${window}_limit_usd`]) }}</span>
            </div>
            <progress v-if="(quota[`${window}_limit_usd`] ?? 0) > 0" :aria-label="`${platformLabel(quota.platform)} ${t(`dashboard.platformQuota.${window}`)}`" :value="Math.max(0, quota[`${window}_usage_usd`] ?? 0)" :max="quota[`${window}_limit_usd`] ?? 1" class="h-1.5 w-full accent-emerald-500" />
            <p v-if="quota[`${window}_window_resets_at`]" class="text-gray-500 dark:text-gray-400">{{ t('dashboard.platformQuota.resetsAt', { time: formatDateTimeToMinute(quota[`${window}_window_resets_at`]) }) }}</p>
          </div>
        </template>
      </article>
    </div>
  </details>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserDashboardStats } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'
import { formatCurrency, formatDateTimeToMinute, formatNumber } from '@/utils/format'
import { platformLabel } from '@/utils/platformColors'

const props = defineProps<{
  stats: UserDashboardStats | null
  balance: number | null
  concurrency?: number
  isSimple: boolean
  canRecharge: boolean
  platformQuotas: PlatformQuotaItem[]
  refreshing: boolean
}>()
defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const windows = ['daily', 'weekly', 'monthly'] as const
const limitedQuotas = computed(() => props.platformQuotas.filter(quota => windows.some(window => quota[`${window}_limit_usd`] != null)))
</script>

<style scoped>
.summary-tile { @apply min-h-28 min-w-0 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800; }
.summary-tile--balance { @apply bg-emerald-50/50 dark:bg-emerald-900/10; }
.summary-label { @apply flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400; }
.summary-value { @apply mt-3 break-words text-2xl font-semibold tabular-nums; }
.summary-detail { @apply mt-1 text-xs text-gray-500 dark:text-gray-400; }
</style>
