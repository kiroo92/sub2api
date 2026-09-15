<template>
  <article class="min-w-0 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800" :data-package-id="item.id">
    <header class="mb-3 flex items-start justify-between gap-2">
      <div class="min-w-0 break-words"><h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ item.plan.name }}</h3><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.plan.group_name }} · #{{ item.id }}</p></div>
      <span class="badge shrink-0" :class="state === 'available' ? 'badge-success' : 'badge-gray'">{{ t(`packages.${state}`) }}</span>
    </header>
    <div v-if="period" class="space-y-2">
      <div class="flex flex-wrap items-baseline justify-between gap-2"><span class="text-xs text-gray-500 dark:text-gray-400">{{ t('dashboard.remainingQuota') }}</span><strong class="text-xl font-semibold tabular-nums text-emerald-700 dark:text-emerald-400">{{ formatCurrency(Math.max(0, period.quota_usd - period.used_usd)) }}</strong></div>
      <div class="flex flex-wrap justify-between gap-1 text-xs"><span>{{ t('packages.period', { index: item.periods.findIndex(entry => entry.id === period?.id) + 1, count: item.periods.length }) }}</span><span>{{ t('packages.used', { used: period.used_usd.toFixed(2), quota: period.quota_usd.toFixed(2) }) }}</span></div>
      <progress :aria-label="t('packages.current')" :value="Math.min(period.used_usd, period.quota_usd)" :max="period.quota_usd || 1" class="h-1.5 w-full accent-emerald-500" />
      <p class="text-xs text-gray-500">{{ t(nextPeriod ? 'packages.nextReset' : 'packages.noNextPeriod') }}{{ nextPeriod ? ': ' + formatDateTimeToMinute(nextPeriod.starts_at) : '' }}</p>
    </div>
    <p class="mt-3 text-xs text-gray-500">{{ t(item.plan.validity_days === 7 ? 'packages.weekSchedule' : 'packages.monthSchedule') }}</p>
    <div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-3 text-xs dark:border-dark-700">
      <span>{{ t('userSubscriptions.expires') }}: {{ formatDateTimeToMinute(item.expires_at) }}</span>
      <RouterLink v-if="item.group_buy_id" :to="`/package-groups/${item.group_buy_id}`" class="text-primary-600">{{ t('packages.groupNumber', { id: item.group_buy_id }) }}</RouterLink>
    </div>
    <slot />
  </article>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useNow } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { UserPackage } from '@/types/packages'
import { formatCurrency, formatDateTimeToMinute } from '@/utils/format'
const props = defineProps<{ item: UserPackage }>()
const { t } = useI18n()
const now = useNow({ interval: 1000 })
const period = computed(() => props.item.periods.find(entry => Date.parse(entry.starts_at) <= now.value.getTime() && Date.parse(entry.ends_at) > now.value.getTime()) ?? null)
const nextPeriod = computed(() => props.item.periods.find(entry => Date.parse(entry.starts_at) > now.value.getTime()))
const state = computed(() => {
  if (Date.parse(props.item.expires_at) <= now.value.getTime()) return 'expired'
  if (props.item.status !== 'active') return 'inactive'
  return period.value && period.value.used_usd < period.value.quota_usd ? 'available' : 'exhausted'
})
</script>
