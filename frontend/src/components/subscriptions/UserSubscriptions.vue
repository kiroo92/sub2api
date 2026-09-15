<template>
  <section id="subscriptions" class="scroll-mt-20 space-y-4" aria-labelledby="subscriptions-heading">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h2 id="subscriptions-heading" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('userSubscriptions.title') }}</h2>
      <RouterLink v-if="canPurchase" to="/purchase?tab=subscription" class="inline-flex items-center gap-1 text-sm font-medium text-primary-600 dark:text-primary-400">
        {{ t('packages.shop') }}<Icon name="arrowRight" size="sm" />
      </RouterLink>
    </header>
    <OwnedPackages ref="packages" @loaded="packageCount = $event; packageLoading = false; packageError = false" @error="packageLoading = false; packageError = true" />
    <p v-if="loading && !subscriptions.length" class="py-4 text-center text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
    <p v-if="subscriptionLoadFailed" role="alert" class="text-sm text-red-600 dark:text-red-400">
      {{ t('userSubscriptions.failedToLoad') }}
      <button type="button" class="underline" :disabled="loading" @click="loadSubscriptions">{{ t('common.refresh') }}</button>
    </p>
    <div v-if="active.length" class="grid items-start gap-4 md:grid-cols-2 xl:grid-cols-3">
      <LegacySubscriptionCard v-for="subscription in active" :key="`legacy:${subscription.id}`" :subscription="subscription" />
    </div>
    <details v-if="history.length" class="text-sm text-gray-500 dark:text-gray-400">
      <summary class="cursor-pointer">{{ t('packages.legacyHistory') }} ({{ history.length }})</summary>
      <div class="mt-3 grid items-start gap-4 md:grid-cols-2 xl:grid-cols-3">
        <LegacySubscriptionCard v-for="subscription in history" :key="`legacy:${subscription.id}`" :subscription="subscription" />
      </div>
    </details>
    <p v-if="!loading && !subscriptionLoadFailed && !packageLoading && !packageError && !packageCount && !subscriptions.length" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('packages.emptySubscriptions') }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useNow } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import OwnedPackages from '@/components/packages/OwnedPackages.vue'
import LegacySubscriptionCard from './LegacySubscriptionCard.vue'
import Icon from '@/components/icons/Icon.vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()
const packages = ref<InstanceType<typeof OwnedPackages>>()
const subscriptions = ref<UserSubscription[]>([])
const packageCount = ref(0)
const packageLoading = ref(true)
const packageError = ref(false)
const loading = ref(false)
const subscriptionLoadFailed = ref(false)
const now = useNow({ interval: 1000 })
const canPurchase = computed(() => isFeatureFlagEnabled(FeatureFlags.payment))
const active = computed(() => subscriptions.value.filter(item => item.status === 'active' && (!item.expires_at || Date.parse(item.expires_at) > now.value.getTime())))
const history = computed(() => subscriptions.value.filter(item => !active.value.includes(item)))
let refreshPending = false

async function loadSubscriptions() {
  if (loading.value) { refreshPending = true; return }
  loading.value = true
  try {
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
    subscriptionLoadFailed.value = false
  } catch {
    subscriptionLoadFailed.value = true
  } finally {
    loading.value = false
    if (refreshPending) { refreshPending = false; void loadSubscriptions() }
  }
}

function refresh() {
  void packages.value?.refresh()
  void loadSubscriptions()
}
defineExpose({ refresh })
onMounted(loadSubscriptions)
</script>
