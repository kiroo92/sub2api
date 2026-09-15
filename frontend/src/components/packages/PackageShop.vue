<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between gap-3">
      <h2 class="text-xl font-bold">{{ t('packages.shop') }}</h2>
      <RouterLink to="/package-groups" class="btn btn-secondary">{{ t('packages.hall') }}</RouterLink>
    </div>
    <PackageRules />
    <p v-if="loading" class="py-12 text-center">{{ t('common.loading') }}</p>
    <div v-else-if="error" role="alert" class="card p-6 text-center"><p>{{ error }}</p><button class="btn btn-secondary mt-3" @click="load">{{ t('common.refresh') }}</button></div>
    <p v-else-if="!plans.length" class="card py-12 text-center text-gray-500">{{ t('packages.emptyPlans') }}</p>
    <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <PackagePlanCard v-for="plan in plans" :key="plan.id" :plan="plan">
        <template #actions>
          <button class="btn btn-secondary flex-1" :disabled="starting !== null" @click="$emit('select', plan)">{{ t('packages.single') }}</button>
          <button v-if="plan.group_buy_enabled" class="btn btn-primary flex-1" :disabled="starting !== null" @click="start(plan)">{{ starting === plan.id ? t('common.processing') : t('packages.start') }}</button>
        </template>
      </PackagePlanCard>
    </div>
  </div>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { packagesAPI } from '@/api/packages'
import type { PackagePlan } from '@/types/packages'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import PackagePlanCard from './PackagePlanCard.vue'
import PackageRules from './PackageRules.vue'
const emit = defineEmits<{ select: [plan: PackagePlan]; start: [plan: PackagePlan, group: Awaited<ReturnType<typeof packagesAPI.startGroup>>] }>()
const { t } = useI18n()
const app = useAppStore()
const plans = ref<PackagePlan[]>([])
const loading = ref(true)
const error = ref('')
const starting = ref<number | null>(null)
async function load() {
  loading.value = true
  error.value = ''
  try { plans.value = await packagesAPI.plans() }
  catch (err) { error.value = extractApiErrorMessage(err, t('packages.loadError')) }
  finally { loading.value = false }
}
async function start(plan: PackagePlan) {
  if (starting.value !== null) return
  starting.value = plan.id
  try {
    const group = await packagesAPI.startGroup(plan.id)
    emit('start', plan, group)
  }
  catch (err) { app.showError(extractApiErrorMessage(err, t('common.error'))) }
  finally { starting.value = null }
}
onMounted(load)
</script>
