<template>
  <article class="subscription-plan flex min-w-0 flex-col rounded-lg border border-gray-200 bg-white p-5 transition-colors hover:border-primary-300 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-600">
    <div class="mb-4 flex items-center justify-between gap-3">
      <span :class="['inline-flex shrink-0 rounded-md px-2 py-1 text-xs font-medium', badgeLightClass]">{{ pLabel }}</span>
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.planCard.validity') }} · {{ validitySuffix }}</span>
    </div>
    <div class="min-w-0">
      <h3
        :title="plan.name"
        class="min-h-14 min-w-0 break-words [overflow-wrap:anywhere] text-lg font-semibold leading-7 text-gray-900 dark:text-white line-clamp-2"
      >
        {{ plan.name }}
      </h3>
      <p v-if="plan.description" class="mt-2 min-h-10 text-sm leading-5 text-gray-500 dark:text-gray-400 line-clamp-2" :title="plan.description">
        {{ plan.description }}
      </p>
    </div>
    <div class="my-5">
      <div class="flex flex-wrap items-baseline gap-x-1.5 text-gray-900 dark:text-white">
        <span class="text-lg font-medium">{{ planCurrencySymbol }}</span>
        <span class="text-3xl font-semibold tabular-nums">{{ plan.price }}</span>
        <span v-if="plan.currency" class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ plan.currency }}</span>
        <span class="ml-1 text-sm text-gray-500 dark:text-gray-400">/ {{ validitySuffix }}</span>
      </div>
      <div v-if="plan.original_price" class="mt-2 flex items-center gap-2">
        <span class="text-xs text-gray-400 line-through dark:text-dark-500">{{ planCurrencySymbol }}{{ plan.original_price }}<template v-if="plan.currency"> {{ plan.currency }}</template></span>
        <span :class="['rounded px-1 py-0.5 text-[10px] font-semibold', discountClass]">{{ discountText }}</span>
      </div>
    </div>

    <div class="plan-quotas mb-5 grid grid-cols-2 gap-x-5 gap-y-4 border-y border-gray-100 py-4 text-sm dark:border-dark-700">
      <div class="flex items-center justify-between">
        <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.rate') }}</span>
        <span class="font-medium text-gray-700 dark:text-gray-300">{{ rateDisplay }}</span>
      </div>
      <div v-if="hasPeakRate" class="col-span-2 flex items-center justify-between gap-2">
        <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.peakRate') }}</span>
        <span class="text-right font-medium text-amber-700 dark:text-amber-300">{{ peakRateDisplay }}</span>
      </div>
      <div v-if="plan.daily_limit_usd != null" class="flex items-center justify-between">
        <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.dailyLimit') }}</span>
        <span class="font-medium text-gray-700 dark:text-gray-300">${{ plan.daily_limit_usd }}</span>
      </div>
      <div v-if="plan.weekly_limit_usd != null" class="flex items-center justify-between">
        <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.weeklyLimit') }}</span>
        <span class="font-medium text-gray-700 dark:text-gray-300">${{ plan.weekly_limit_usd }}</span>
      </div>
      <div v-if="plan.monthly_limit_usd != null" class="flex items-center justify-between">
        <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.monthlyLimit') }}</span>
        <span class="font-medium text-gray-700 dark:text-gray-300">${{ plan.monthly_limit_usd }}</span>
      </div>
      <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="flex items-center justify-between">
        <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.quota') }}</span>
        <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.planCard.unlimited') }}</span>
      </div>
      <div v-if="modelScopeLabels.length > 0" class="col-span-2 flex items-center justify-between">
        <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.models') }}</span>
        <div class="flex flex-wrap justify-end gap-1">
          <span v-for="scope in modelScopeLabels" :key="scope"
                class="rounded bg-gray-200/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
            {{ scope }}
          </span>
        </div>
      </div>
    </div>

    <!-- Features list (compact) -->
    <div v-if="plan.features.length > 0" class="mb-6 space-y-2.5">
      <div v-for="feature in plan.features" :key="feature" class="flex items-start gap-1.5">
        <Icon name="check" size="sm" :class="['mt-0.5 flex-shrink-0', iconClass]" aria-hidden="true" />
        <span class="min-w-0 break-words text-sm text-gray-600 dark:text-gray-300">{{ feature }}</span>
      </div>
    </div>

    <div class="flex-1" />

    <!-- Subscribe Button -->
    <button
      type="button"
      class="flex w-full items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 py-3 text-sm font-medium text-white transition-colors hover:bg-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
      @click="emit('select', plan)"
    >
      {{ t('payment.subscribeNow') }}
      <Icon name="chevronRight" size="sm" aria-hidden="true" />
    </button>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { planValiditySuffix } from './validity'
import { currencySymbol } from '@/components/payment/currency'
import {
  platformBadgeLightClass,
  platformIconClass,
  platformDiscountClass,
  platformLabel,
} from '@/utils/platformColors'

const props = defineProps<{ plan: SubscriptionPlan }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const platform = computed(() => props.plan.group_platform || '')

// Derived color classes from central config
const badgeLightClass = computed(() => platformBadgeLightClass(platform.value))
const iconClass = computed(() => platformIconClass(platform.value))
const discountClass = computed(() => platformDiscountClass(platform.value))
const pLabel = computed(() => platformLabel(platform.value))

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

const appStore = useAppStore()
const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
  return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  if (platform.value !== 'antigravity') return []
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => planValiditySuffix(props.plan, t))
</script>

<style scoped>
.plan-quotas > div { flex-direction: column; align-items: flex-start; gap: 4px; }
.plan-quotas > div > span:first-child { color: #64748b; font-size: 12px; }
.plan-quotas > div > span:last-child { font-size: 15px; font-variant-numeric: tabular-nums; }
.dark .plan-quotas > div > span:first-child { color: #94a3b8; }
@media (prefers-reduced-motion: reduce) { .subscription-plan, .subscription-plan button { transition: none; } }
</style>
