<template>
  <AppLayout>
    <div class="mx-auto min-w-0 space-y-5" :class="detailID ? 'w-full' : 'max-w-6xl'">
      <div v-if="detailID" class="flex flex-wrap items-center justify-between gap-3 text-xs text-gray-500 dark:text-gray-400">
        <nav :aria-label="t('packages.detail')" class="flex min-w-0 items-center gap-2">
          <RouterLink to="/purchase?tab=subscription" class="hover:text-emerald-600">{{ t('packages.shop') }}</RouterLink>
          <Icon name="chevronRight" size="xs" aria-hidden="true" />
          <span aria-current="page">{{ t(groups[0]?.status === 'draft' ? 'packages.start' : 'packages.detail') }}</span>
        </nav>
        <div class="flex items-center gap-3">
          <RouterLink to="/package-groups" class="flex items-center gap-1 hover:text-emerald-600"><Icon name="arrowLeft" size="xs" aria-hidden="true" />{{ t('packages.back') }}</RouterLink>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg hover:bg-gray-100 disabled:opacity-50 dark:hover:bg-dark-700" :aria-label="t('common.refresh')" :title="t('common.refresh')" :disabled="loading" @click="load(true)"><Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" /></button>
        </div>
      </div>
      <div v-else class="flex flex-wrap items-center justify-between gap-3">
        <h1 class="text-2xl font-bold">{{ t('packages.hall') }}</h1>
        <div class="flex flex-wrap gap-2"><RouterLink to="/purchase?tab=subscription" class="btn btn-secondary">{{ t('packages.shop') }}</RouterLink><button class="btn btn-secondary" :aria-label="t('common.refresh')" :disabled="loading" @click="load(true)">{{ t('common.refresh') }}</button></div>
      </div>
      <PackageRules v-if="!detailID" />
      <p v-if="loading && !groups.length" class="py-12 text-center">{{ t('common.loading') }}</p>
      <p v-if="error" role="alert" class="rounded-xl border border-red-200 p-4 text-sm text-red-600">{{ error }} <button class="underline" :disabled="loading" @click="load(true)">{{ t('common.refresh') }}</button></p>
      <p v-if="!loading && !error && !groups.length" class="card py-12 text-center text-gray-500">{{ t('packages.emptyGroups') }}</p>
      <PackageGroupDetail v-if="detailID && groups[0]" :group="groups[0]" :accepted="accepted" @update:accepted="accepted = $event" @checkout="checkout(groups[0])" />
      <div v-if="groups.length && !detailID" class="grid gap-4 sm:grid-cols-2">
        <article v-for="group in groups" :key="group.id" class="package-group-card min-w-0 rounded-2xl border bg-white p-5 shadow-sm dark:bg-dark-800" :data-theme="themeID(group)" :style="themeStyle(group)">
          <div class="min-w-0 space-y-5">
            <header class="flex flex-wrap items-start justify-between gap-3">
              <div class="flex min-w-0 items-start gap-3">
                <span class="package-group-card__icon flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"><Icon name="users" size="md" aria-hidden="true" /></span>
                <div class="min-w-0 break-words"><h2 class="text-lg font-bold">{{ group.plan.name }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('packages.groupNumber', { id: group.id }) }} · {{ t(`packages.${group.status}`) }}</p></div>
              </div>
              <span class="package-group-card__status badge">{{ t(group.joined ? 'packages.joined' : `packages.${group.status}`) }}</span>
            </header>
            <div>
              <div class="mb-2 flex justify-between text-sm"><span>{{ t('packages.target') }}</span><strong class="package-group-card__accent">{{ t('packages.members', { count: group.paid_count, target: group.target_members }) }}</strong></div>
              <progress :aria-label="t('packages.target')" :value="group.paid_count" :max="Math.max(1, group.target_members)" class="package-group-card__progress h-2 w-full" />
            </div>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
              <div class="package-group-card__metric rounded-xl border p-3"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('packages.base') }}</p><strong class="mt-1 block">${{ group.plan.base_quota_usd.toLocaleString() }}</strong></div>
              <div class="package-group-card__metric rounded-xl border p-3"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t(group.status === 'settled' ? 'packages.finalQuota' : 'packages.expectedQuota') }}</p><strong class="mt-1 block">${{ reachedQuota(group).toLocaleString() }}</strong></div>
              <div class="package-group-card__metric rounded-xl border p-3"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('packages.nextQuota') }}</p><strong class="mt-1 block">{{ nextTier(group) ? '$' + nextTier(group)!.quota_usd.toLocaleString() : '—' }}</strong></div>
            </div>
            <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400"><span>{{ t('packages.duration', { hours: group.plan.group_buy_hours }) }}</span><span v-if="group.ends_at && group.status !== 'settled'" class="package-group-card__accent font-mono font-semibold">{{ t('packages.remaining', { time: remaining(group.ends_at) }) }}</span></div>
            <p v-if="group.ends_at" class="text-xs text-gray-500 dark:text-gray-400">{{ t('packages.deadline') }}: {{ formatDateTimeToMinute(group.ends_at) }}</p>
            <RouterLink :to="`/package-groups/${group.id}`" class="package-group-card__button btn w-full">{{ t(group.joined ? 'packages.joined' : 'packages.detail') }}</RouterLink>
          </div>
        </article>
      </div>
    </div>
    <PackagePaymentDialog v-if="paymentGroup" :key="paymentGroup.id" :show="showPayment" :group="paymentGroup" terms-accepted @close="showPayment = false" @success="load(true)" />
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useIntervalFn, useNow } from '@vueuse/core'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { packagesAPI } from '@/api/packages'
import type { PackageGroupBuy } from '@/types/packages'
import { PACKAGE_THEME_COLORS, resolvePackageThemeColor } from '@/types/packages'
import AppLayout from '@/components/layout/AppLayout.vue'
import PackageRules from '@/components/packages/PackageRules.vue'
import PackageGroupDetail from '@/components/packages/PackageGroupDetail.vue'
import PackagePaymentDialog from '@/components/packages/PackagePaymentDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
const { t } = useI18n()
const route = useRoute()
const now = useNow({ interval: 1000 })
const detailID = computed(() => Number(route.params.id) || null)
const groups = ref<PackageGroupBuy[]>([])
const loading = ref(false)
const error = ref('')
const accepted = ref(false)
const paymentGroup = ref<PackageGroupBuy | null>(null)
const showPayment = ref(false)
let requestSequence = 0
async function load(force = false) {
  if (loading.value && !force) return
  const sequence = ++requestSequence
  const requestedID = detailID.value
  loading.value = true
  try {
    const result = requestedID ? [await packagesAPI.group(requestedID)] : await packagesAPI.groups()
    if (sequence === requestSequence && requestedID === detailID.value) { groups.value = result; error.value = '' }
  } catch (err) {
    if (sequence === requestSequence && requestedID === detailID.value) error.value = extractApiErrorMessage(err, t('packages.loadError'))
  }
  finally { if (sequence === requestSequence) loading.value = false }
}
function reachedQuota(group: PackageGroupBuy) {
  return group.final_quota_usd ?? group.plan.tiers.reduce((quota, tier) => group.paid_count >= tier.members ? tier.quota_usd : quota, group.plan.base_quota_usd)
}
function nextTier(group: PackageGroupBuy) { return group.status === 'settled' ? undefined : group.plan.tiers.find(tier => tier.members > group.paid_count) }
function themeID(group: PackageGroupBuy) { return resolvePackageThemeColor(group.plan.theme_color, group.plan.id) }
function themeStyle(group: PackageGroupBuy) {
  const theme = PACKAGE_THEME_COLORS[themeID(group)]
  return { '--package-accent': theme.accent, '--package-soft': theme.soft, '--package-border': theme.border }
}
function remaining(endsAt: string) {
  const seconds = Math.max(0, Math.floor((Date.parse(endsAt) - now.value.getTime()) / 1000))
  return [Math.floor(seconds / 3600), Math.floor(seconds / 60) % 60, seconds % 60].map(value => String(value).padStart(2, '0')).join(':')
}
function canJoin(group: PackageGroupBuy, at = now.value.getTime()) {
  return !group.joined && !group.order_id && group.status !== 'settled' && (!group.ends_at || Date.parse(group.ends_at) > at)
}
function checkout(group: PackageGroupBuy) {
  if (!group.order_id && (!accepted.value || !canJoin(group, Date.now()))) return
  paymentGroup.value = group
  showPayment.value = true
}
watch(detailID, () => { accepted.value = false; groups.value = []; error.value = ''; void load(true) }, { immediate: true })
useIntervalFn(load, 15000)
</script>

<style scoped>
.package-group-card {
  border-color: var(--package-border);
}

.package-group-card__icon {
  background: var(--package-soft);
  color: var(--package-accent);
}

.package-group-card__accent {
  color: var(--package-accent);
}

.package-group-card__status {
  color: var(--package-accent);
  background: var(--package-soft);
}

.package-group-card__metric {
  border-color: var(--package-border);
}

.package-group-card__progress {
  accent-color: var(--package-accent);
}

.package-group-card__button {
  background: var(--package-accent);
  color: white;
}

.package-group-card__button:hover {
  filter: brightness(0.92);
}
</style>
