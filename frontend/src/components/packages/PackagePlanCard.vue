<template>
  <article class="flex h-full min-w-0 flex-col overflow-hidden rounded-2xl border border-primary-200 bg-white shadow-sm dark:border-primary-900 dark:bg-dark-800">
    <div class="flex-1 space-y-4 p-5">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="min-w-0 break-words text-lg font-bold text-gray-900 dark:text-white">{{ plan.name }}</h3>
        <span class="badge badge-primary">{{ t(plan.validity_days === 7 ? 'packages.week' : 'packages.month') }}</span>
      </div>
      <p class="break-words text-xs text-gray-500 dark:text-gray-400">{{ plan.group_name }} · {{ platformLabel(plan.group_platform) }}</p>
      <p class="text-3xl font-bold text-primary-600 dark:text-primary-400">{{ formatPaymentAmount(plan.price, plan.currency) }}</p>
      <div class="flex flex-wrap gap-2 text-xs">
        <span class="rounded-full bg-primary-50 px-2 py-1 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ t('packages.validity', { days: plan.validity_days }) }}</span>
        <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-700">{{ t('packages.periodQuota', { amount: quota(plan.base_quota_usd / (plan.validity_days === 30 ? 4 : 1)) }) }}</span>
      </div>
      <p v-if="plan.description" class="whitespace-pre-line break-words text-sm text-gray-500 dark:text-gray-400">{{ plan.description }}</p>
      <div class="overflow-hidden rounded-xl border border-primary-100 text-sm dark:border-dark-600">
        <p class="bg-primary-50 px-3 py-2 font-semibold dark:bg-primary-900/20">{{ t('packages.tiers') }}</p>
        <div class="flex justify-between gap-2 px-3 py-3"><span>{{ t('packages.single') }}</span><strong>{{ t('packages.quota', { amount: quota(plan.base_quota_usd) }) }}</strong></div>
        <div v-for="tier in plan.group_buy_enabled ? plan.tiers : []" :key="tier.members" class="flex justify-between gap-2 border-t border-gray-100 px-3 py-3 dark:border-dark-700">
          <span>{{ t('packages.tier', { members: tier.members }) }}</span><strong class="text-primary-600 dark:text-primary-400">{{ t('packages.quota', { amount: quota(tier.quota_usd) }) }}</strong>
        </div>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(plan.validity_days === 7 ? 'packages.weekSchedule' : 'packages.monthSchedule') }}</p>
    </div>
    <div v-if="$slots.actions" class="flex gap-2 border-t border-gray-100 p-4 dark:border-dark-700"><slot name="actions" /></div>
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PackagePlan } from '@/types/packages'
import { formatPaymentAmount } from '@/components/payment/currency'
import { platformLabel } from '@/utils/platformColors'
defineProps<{ plan: PackagePlan }>()
const { t } = useI18n()
const quota = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 8 })
</script>
