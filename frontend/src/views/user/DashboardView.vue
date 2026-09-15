<template>
  <AppLayout>
    <div class="space-y-5">
      <UserDashboardStats :stats="stats" :balance="user?.balance ?? null" :concurrency="user?.concurrency" :is-simple="authStore.isSimpleMode" :can-recharge="canRecharge" :platform-quotas="platformQuotas" :refreshing="refreshing" @refresh="refreshAll" />
      <p v-if="statsFailed || accountFailed || quotasFailed" role="alert" class="text-sm text-red-600 dark:text-red-400">
        {{ [statsFailed && t('dashboard.statsLoadFailed'), accountFailed && t('dashboard.accountLoadFailed'), quotasFailed && t('dashboard.quotasLoadFailed')].filter(Boolean).join(' ') }}
        <button type="button" :disabled="refreshing" class="underline" @click="refreshAll">{{ t('common.refresh') }}</button>
      </p>
      <template v-if="!authStore.isSimpleMode">
        <UserSubscriptions ref="subscriptions" />
        <RedeemSection @refresh="refreshEntitlements" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import { getMyPlatformQuotas } from '@/api/user'
import { paymentAPI } from '@/api/payment'
import type { PlatformQuotaItem } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserSubscriptions from '@/components/subscriptions/UserSubscriptions.vue'
import RedeemSection from '@/components/user/RedeemSection.vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()
const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const platformQuotas = ref<PlatformQuotaItem[]>([])
const subscriptions = ref<InstanceType<typeof UserSubscriptions>>()
const refreshing = ref(false)
const statsFailed = ref(false)
const accountFailed = ref(false)
const quotasFailed = ref(false)
const rechargeEnabled = ref(false)
const paymentEnabled = computed(() => !authStore.isSimpleMode && isFeatureFlagEnabled(FeatureFlags.payment))
const canRecharge = computed(() => paymentEnabled.value && rechargeEnabled.value)
let refreshPending = false

async function loadSummary(refreshAccount = true) {
  if (refreshing.value) { refreshPending = true; return }
  refreshing.value = true
  const [usage, account, quotas] = await Promise.allSettled([
    usageAPI.getDashboardStats(),
    refreshAccount ? authStore.refreshUser() : Promise.resolve(),
    authStore.isSimpleMode ? Promise.resolve(null) : getMyPlatformQuotas()
  ])
  statsFailed.value = usage.status === 'rejected'
  if (usage.status === 'fulfilled') stats.value = usage.value
  if (refreshAccount) accountFailed.value = account.status === 'rejected'
  quotasFailed.value = quotas.status === 'rejected'
  if (quotas.status === 'fulfilled') platformQuotas.value = quotas.value?.platform_quotas ?? []
  refreshing.value = false
  if (refreshPending) { refreshPending = false; void loadSummary() }
}

function refreshEntitlements(accountRefreshed: boolean) {
  accountFailed.value = !accountRefreshed
  subscriptions.value?.refresh()
  void loadSummary(false)
}
function refreshAll() {
  subscriptions.value?.refresh()
  void loadSummary()
}
watch(paymentEnabled, async (enabled, _previous, onCleanup) => {
  let stale = false
  onCleanup(() => { stale = true })
  rechargeEnabled.value = false
  if (!enabled) return
  try {
    const { data } = await paymentAPI.getConfig()
    if (!stale) rechargeEnabled.value = data.enabled && !data.balance_disabled
  } catch { /* The common checkout remains available through the shop. */ }
}, { immediate: true })
onMounted(() => { void loadSummary() })
</script>
