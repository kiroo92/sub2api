<template>
  <div class="space-y-3">
    <p v-if="statsError" role="alert" class="text-sm text-red-600 dark:text-red-400">
      {{ t('dashboard.statsLoadFailed') }}
      <button class="ml-2 underline" :disabled="loading" @click="$emit('retryStats')">{{ t('common.refresh') }}</button>
    </p>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2" :class="isSimple ? 'xl:grid-cols-3' : 'xl:grid-cols-4'">
      <article data-testid="today-cost" class="summary-card border-violet-100 bg-violet-50/70 dark:border-violet-900/50 dark:bg-violet-900/10" :aria-busy="loading">
        <p class="text-sm text-violet-700 dark:text-violet-300">{{ t('dashboard.todayCost') }}</p>
        <p class="summary-value">{{ stats ? '$' + formatCost(stats.today_actual_cost) : '—' }}</p>
        <p v-if="stats && costSplitAvailable" class="summary-note">{{ t('dashboard.subscriptionCost') }} ${{ formatCost(stats.today_subscription_cost!) }} · {{ t('dashboard.balanceCost') }} ${{ formatCost(stats.today_balance_cost!) }}</p>
        <p v-else class="summary-note">{{ t(loading ? 'common.loading' : 'dashboard.actualCostHint') }}</p>
      </article>
      <article v-if="!isSimple" data-testid="wallet-balance" class="summary-card border-emerald-100 bg-emerald-50/70 dark:border-emerald-900/50 dark:bg-emerald-900/10" :aria-busy="balanceLoading">
        <div class="flex items-center justify-between gap-2">
          <p class="text-sm text-emerald-700 dark:text-emerald-300">{{ t('dashboard.walletBalance') }}</p>
          <router-link v-if="canRecharge" to="/purchase?tab=recharge" class="rounded-lg border border-emerald-200 px-2 py-1 text-xs text-emerald-700 hover:bg-emerald-100 dark:border-emerald-800 dark:text-emerald-300 dark:hover:bg-emerald-900/30">{{ t('nav.recharge') }}</router-link>
        </div>
        <p class="summary-value">{{ balance != null ? '$' + formatBalance(balance) : '—' }}</p>
        <p v-if="balanceError" role="alert" class="summary-note text-red-600">{{ t('dashboard.balanceLoadFailed') }} <button class="underline" :disabled="balanceLoading" @click="$emit('retryBalance')">{{ t('common.refresh') }}</button></p>
        <p v-else class="summary-note">{{ t(balanceLoading ? 'common.loading' : 'common.available') }}</p>
      </article>
      <article data-testid="today-requests" class="summary-card border-blue-100 bg-blue-50/70 dark:border-blue-900/50 dark:bg-blue-900/10" :aria-busy="loading">
        <p class="text-sm text-blue-700 dark:text-blue-300">{{ t('dashboard.todayRequests') }}</p>
        <p class="summary-value">{{ stats ? formatNumber(stats.today_requests) : '—' }}</p>
        <p class="summary-note">{{ stats ? t('common.total') + ': ' + formatNumber(stats.total_requests) : t(loading ? 'common.loading' : 'dashboard.unavailable') }}</p>
      </article>
      <article data-testid="today-tokens" class="summary-card border-amber-100 bg-amber-50/70 dark:border-amber-900/50 dark:bg-amber-900/10" :aria-busy="loading">
        <p class="text-sm text-amber-700 dark:text-amber-300">{{ t('dashboard.todayTokens') }}</p>
        <p class="summary-value">{{ stats ? formatTokens(stats.today_tokens) : '—' }}</p>
        <p v-if="stats" class="summary-note">{{ t('dashboard.input') }} {{ formatTokens(stats.today_input_tokens) }} · {{ t('dashboard.output') }} {{ formatTokens(stats.today_output_tokens) }} · {{ t('dashboard.cache') }} {{ formatTokens(stats.today_cache_creation_tokens + stats.today_cache_read_tokens) }}</p>
        <p v-else class="summary-note">{{ t(loading ? 'common.loading' : 'dashboard.unavailable') }}</p>
      </article>
    </div>
  </div>

  <slot />

  <!-- Optional platform detail stays below the subscription overview. -->
  <details v-if="!isSimple && (platformCards.length > 0 || quotasError || quotasLoading)" class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
    <summary class="cursor-pointer text-sm text-gray-600 dark:text-gray-300">
      <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.platformBreakdown') }}</span>
      <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('dashboard.platformCount', { count: platformCount }) }}
      </span>
    </summary>
    <p v-if="quotasLoading" class="mt-3 text-sm text-gray-500">{{ t('common.loading') }}</p>
    <p v-else-if="quotasError" role="alert" class="mt-3 text-sm text-red-600">{{ t('dashboard.quotasLoadFailed') }} <button class="underline" @click="$emit('retryQuotas')">{{ t('common.refresh') }}</button></p>
    <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div
        v-for="item in platformCards"
        :key="item.platform"
        data-testid="platform-card"
        :data-platform="item.platform"
        :class="[
          'rounded-lg border p-3',
          item.isOther
            ? 'border-dashed border-gray-300 bg-gray-50 dark:border-dark-500 dark:bg-dark-700/30'
            : 'border-gray-200 dark:border-dark-600'
        ]"
      >
        <div class="flex items-center justify-between">
          <span class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ item.isOther ? t('dashboard.platformOther') : platformLabel(item.platform) }}
          </span>
          <span v-if="stats" class="font-mono text-sm text-purple-600 dark:text-purple-400" :title="t('dashboard.actual')">
            ${{ formatCost(item.total_actual_cost) }}
          </span>
        </div>
        <div v-if="stats" class="mt-2 space-y-1 text-xs">
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">{{ t('dashboard.todayCost') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">${{ formatCost(item.today_actual_cost) }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">{{ t('dashboard.requests') }}</span>
            <span class="font-mono text-gray-700 dark:text-gray-300">
              {{ item.total_requests > 0 ? formatNumber(item.total_requests) : '-' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">{{ t('dashboard.tokens') }}</span>
            <span class="font-mono text-gray-700 dark:text-gray-300">
              {{ item.total_tokens > 0 ? formatTokens(item.total_tokens) : '-' }}
            </span>
          </div>
        </div>

        <!-- Quota 区：仅当 quota 配置存在、非 __other__ 且至少有一个窗口配了 limit 时显示 -->
        <div v-if="hasAnyLimit(item.quota) && !item.isOther" class="mt-3 space-y-1.5 border-t border-gray-200 pt-2 dark:border-dark-700">
          <p class="text-[10px] uppercase tracking-wide text-gray-400">
            {{ t('dashboard.platformQuota.title') }}
          </p>
          <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
            <div v-if="quotaVal(item.quota, `${w}_limit_usd`) != null" class="space-y-0.5">
              <!-- limit=0：完全禁用 -->
              <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-600 dark:text-gray-300">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="font-mono text-red-500">{{ t('dashboard.platformQuota.disabled') }}</span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                  <div class="h-full w-full rounded-full bg-red-500" />
                </div>
              </template>
              <!-- limit>0：正常用量进度条 -->
              <template v-else>
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-600 dark:text-gray-300">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="font-mono text-gray-700 dark:text-gray-200">
                    ${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}
                  </span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="quotaBarClass(calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number))"
                    :style="{ width: calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number) + '%' }"
                  />
                </div>
                <p v-if="quotaVal(item.quota, `${w}_window_resets_at`)" class="text-[10px] text-gray-400">
                  {{ t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) }}
                </p>
              </template>
            </div>
          </template>
        </div>
      </div>
    </div>
  </details>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PlatformDashboardStats, UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

interface FusedPlatformCard {
  platform: string
  total_actual_cost: number
  today_actual_cost: number
  total_requests: number
  total_tokens: number
  isOther?: boolean
  quota?: PlatformQuotaItem
}

const props = defineProps<{
  stats: UserStatsType | null
  balance: number | null
  loading?: boolean
  statsError?: boolean
  balanceLoading?: boolean
  balanceError?: boolean
  canRecharge?: boolean
  quotasLoading?: boolean
  quotasError?: boolean
  isSimple: boolean
  platformQuotas?: PlatformQuotaItem[] | null
}>()
defineEmits<{ retryStats: []; retryBalance: []; retryQuotas: [] }>()
const { t } = useI18n()
const costSplitAvailable = computed(() => props.stats?.today_subscription_cost != null && props.stats?.today_balance_cost != null)

const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  grok: 'Grok',
  kimi: 'Kimi',
  zhipu: 'Zhipu GLM',
  deepseek: 'DeepSeek',
  minimax: 'MiniMax',
}

const platformLabel = (p: string) => PLATFORM_LABELS[p] ?? p

// 处理"各平台之和 < 总值"的差值：后端按平台聚合时过滤了无法归属平台的行
// （group 与 account 都缺 platform）。这里把差值作为"其他"卡片显式展示，
// 避免 Row 1 总值与 Row 3 平台拆分加总对不上、用户困惑。
const OTHER_THRESHOLD = 0.0001
const platformCards = computed<FusedPlatformCard[]>(() => {
  // 建立 by_platform Map
  const byPlat = new Map<string, PlatformDashboardStats>()
  for (const item of props.stats?.by_platform ?? []) byPlat.set(item.platform, item)

  // 建立 quota Map。三档全空的记录不产生卡片，挂到卡片上也不渲染配额区。
  const byQuota = new Map<string, PlatformQuotaItem>()
  for (const q of props.platformQuotas ?? []) byQuota.set(q.platform, q)

  // 卡片集合 = 有用量的平台 ∪ 至少配置了一档限额的平台。
  // 三档全空的限额记录等价于不限额，不单独产生卡片。
  // 后端 by_platform / quota 接口均不会返回 platform='__other__'，
  // 无需显式排除；__other__ 由下方差值补差逻辑单独追加。
  const platforms = new Set<string>(byPlat.keys())
  for (const [platform, q] of byQuota) {
    if (hasAnyLimit(q)) platforms.add(platform)
  }

  const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok']
  const cards: FusedPlatformCard[] = []

  for (const p of platforms) {
    const stat = byPlat.get(p)
    cards.push({
      platform: p,
      total_actual_cost: stat?.total_actual_cost ?? 0,
      today_actual_cost: stat?.today_actual_cost ?? 0,
      total_requests: stat?.total_requests ?? 0,
      total_tokens: stat?.total_tokens ?? 0,
      quota: byQuota.get(p),
    })
  }

  // 排序：按 PLATFORM_ORDER，未知平台按名称排序
  cards.sort((a, b) => {
    const ai = PLATFORM_ORDER.indexOf(a.platform)
    const bi = PLATFORM_ORDER.indexOf(b.platform)
    if (ai === -1 && bi === -1) return a.platform.localeCompare(b.platform)
    if (ai === -1) return 1
    if (bi === -1) return -1
    return ai - bi
  })

  // __other__ 补差逻辑：只对 by_platform 有 usage 数据的总和计算
  const total = props.stats?.total_actual_cost ?? 0
  const today = props.stats?.today_actual_cost ?? 0
  const sumTotal = cards.reduce((s, c) => s + c.total_actual_cost, 0)
  const sumToday = cards.reduce((s, c) => s + c.today_actual_cost, 0)
  const diffTotal = Math.max(0, total - sumTotal)
  const diffToday = Math.max(0, today - sumToday)

  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    cards.push({
      platform: '__other__',
      total_actual_cost: diffTotal,
      today_actual_cost: diffToday,
      total_requests: 0,
      total_tokens: 0,
      isOther: true,
    })
  }

  return cards
})

// 标题右侧的平台计数 = 实际渲染的平台卡片数，不含"其他"差额卡。
const platformCount = computed(() => platformCards.value.filter((c) => !c.isOther).length)

// Quota helpers

type QuotaWindow = 'daily' | 'weekly' | 'monthly'
type QuotaField = `${QuotaWindow}_limit_usd` | `${QuotaWindow}_usage_usd` | `${QuotaWindow}_window_resets_at`

function quotaVal(q: PlatformQuotaItem | undefined, key: QuotaField): PlatformQuotaItem[QuotaField] {
  return q?.[key]
}

function hasAnyLimit(q: PlatformQuotaItem | undefined): boolean {
  if (!q) return false
  return q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null
}

function calcPercent(usage: number, limit: number): number {
  if (!limit || limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((usage / limit) * 100)))
}

function quotaBarClass(p: number): string {
  if (p >= 95) return 'bg-red-500'
  if (p >= 75) return 'bg-amber-500'
  return 'bg-green-500'
}

// 与 formatBalance 一致使用 Intl.NumberFormat 做半偶舍入，避免 toFixed 在不同 JS 引擎
// 下偶发截断而非四舍五入（与后端展示精度不一致）。
const usdFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})
function formatUsd(n: number): string {
  if (!Number.isFinite(n)) return '0.00'
  return usdFormatter.format(n)
}

function formatResetTime(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

const formatBalance = (b: number) =>
  new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(b)

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatTokens = (t: number) => {
  if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
  if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
  return t.toString()
}
</script>
<style scoped>
.summary-card { @apply flex min-w-0 flex-col rounded-2xl border p-5; }
.summary-value { @apply my-3 break-words text-3xl font-semibold tabular-nums tracking-tight text-gray-900 dark:text-white; }
.summary-note { @apply mt-auto text-xs leading-5 text-gray-500 dark:text-gray-400; }
</style>
