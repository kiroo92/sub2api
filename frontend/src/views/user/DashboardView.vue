<template>
  <AppLayout>
    <div class="mx-auto max-w-screen-2xl space-y-6">
      <header class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">{{ t('dashboard.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.overviewHint') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <router-link v-if="!authStore.isSimpleMode" to="/usage" class="btn btn-secondary btn-sm">{{ t('dashboard.viewUsage') }}</router-link>
          <button class="btn btn-secondary btn-sm" :disabled="loading || balanceLoading || quotasLoading" @click="refreshSummary">{{ t('dashboard.refreshOverview') }}</button>
        </div>
      </header>
      <UserDashboardStats
        :stats="stats" :balance="balance" :is-simple="authStore.isSimpleMode"
        :loading="loading" :stats-error="statsError" :balance-loading="balanceLoading" :balance-error="balanceError"
        :can-recharge="canRecharge" :platform-quotas="platformQuotas" :quotas-loading="quotasLoading" :quotas-error="quotasError"
        @retry-stats="loadStats" @retry-balance="loadBalance" @retry-quotas="loadPlatformQuotas"
      >
        <DashboardSubscriptions v-if="showSubscriptions" />
      </UserDashboardStats>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { usePaymentStore } from '@/stores/payment'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import { getMyPlatformQuotas } from '@/api/user'
import type { PlatformQuotaItem } from '@/types'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import AppLayout from '@/components/layout/AppLayout.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import DashboardSubscriptions from '@/components/user/dashboard/DashboardSubscriptions.vue'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const appStore = useAppStore()
const paymentStore = usePaymentStore()
const stats = ref<UserStatsType | null>(null)
const balance = ref<number | null>(null)
const loading = ref(false)
const balanceLoading = ref(false)
const quotasLoading = ref(false)
const statsError = ref(false)
const balanceError = ref(false)
const quotasError = ref(false)
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
const settingsReady = ref(false)
const rechargeConfigured = ref(false)
const showSubscriptions = computed(() => settingsReady.value && !authStore.isSimpleMode && resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.subscription))
const showPayment = computed(() => settingsReady.value && !authStore.isSimpleMode && appStore.cachedPublicSettings?.payment_enabled === true)
const canRecharge = computed(() => showPayment.value && rechargeConfigured.value)

async function loadStats() {
  if (loading.value) return
  loading.value = true
  statsError.value = false
  try { stats.value = await usageAPI.getDashboardStats() }
  catch { stats.value = null; statsError.value = true }
  finally { loading.value = false }
}

async function loadBalance() {
  if (balanceLoading.value || authStore.isSimpleMode) return
  balanceLoading.value = true
  balanceError.value = false
  try { balance.value = (await authStore.refreshUser()).balance }
  catch { balance.value = null; balanceError.value = true }
  finally { balanceLoading.value = false }
}

async function loadPlatformQuotas() {
  if (quotasLoading.value || authStore.isSimpleMode) return
  quotasLoading.value = true
  quotasError.value = false
  try { platformQuotas.value = (await getMyPlatformQuotas()).platform_quotas ?? [] }
  catch { platformQuotas.value = null; quotasError.value = true }
  finally { quotasLoading.value = false }
}

async function loadRechargeConfig() {
  rechargeConfigured.value = false
  if (!showPayment.value) return
  const config = await paymentStore.fetchConfig(true)
  rechargeConfigured.value = config?.enabled === true && config.balance_disabled === false
}

function refreshSummary() {
  void loadStats()
  void loadBalance()
  void loadPlatformQuotas()
  void loadRechargeConfig()
}

watch(showPayment, () => { void loadRechargeConfig() })
// The subscription anchor appears after settings load; also handle links on this page.
watch([showSubscriptions, () => route.hash], async ([visible, hash]) => {
  if (visible && hash === '#subscriptions') {
    await nextTick()
    document.getElementById('subscriptions')?.scrollIntoView({ block: 'start' })
  }
})
onMounted(async () => {
  refreshSummary()
  await appStore.fetchPublicSettings()
  settingsReady.value = true
})
</script>
