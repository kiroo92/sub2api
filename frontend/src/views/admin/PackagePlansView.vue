<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex items-center justify-between gap-3"><h1 class="text-xl font-bold">{{ t('packages.admin') }}</h1><button class="btn btn-primary" @click="edit()">{{ t('packages.create') }}</button></div>
      <p class="text-sm text-gray-500">{{ t('packages.adminHint') }}</p>
      <p v-if="error" role="alert" class="text-red-600">{{ error }} <button class="underline" @click="load">{{ t('common.refresh') }}</button></p>
      <div class="card overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="border-b border-gray-100 text-gray-500 dark:border-dark-700"><tr><th class="p-4">{{ t('packages.name') }}</th><th class="p-4">{{ t('packages.group') }}</th><th class="p-4">{{ t('packages.price') }}</th><th class="p-4">{{ t('packages.baseQuota') }}</th><th class="p-4">{{ t('common.status') }}</th><th class="p-4">{{ t('common.actions') }}</th></tr></thead>
          <tbody><tr v-for="plan in plans" :key="plan.id" class="border-b border-gray-100 last:border-0 dark:border-dark-700"><td class="p-4 font-medium"><span class="mr-2 inline-block h-3 w-3 rounded-full align-middle" :style="{ backgroundColor: themeSpec(plan.theme_color, plan.id).accent }" :title="t(`packages.themeColors.${themeSpec(plan.theme_color, plan.id).id}`)"></span>{{ plan.name }}<p class="text-xs font-normal text-gray-500">{{ t('packages.validity', { days: plan.validity_days }) }}</p></td><td class="p-4">{{ plan.group_name || '#' + plan.group_id }}</td><td class="p-4">{{ formatPaymentAmount(plan.price, plan.currency) }}</td><td class="p-4">${{ plan.base_quota_usd }}</td><td class="p-4">{{ t(plan.for_sale ? 'packages.forSale' : 'packages.unpublished') }}</td><td class="p-4"><div class="flex gap-2"><button class="btn btn-secondary" @click="edit(plan)">{{ t('common.edit') }}</button><button v-if="plan.for_sale" class="btn btn-secondary" :disabled="saving" @click="unpublish(plan)">{{ t('packages.unpublish') }}</button></div></td></tr></tbody>
        </table>
        <p v-if="loading || !plans.length" class="p-8 text-center text-gray-500">{{ t(loading ? 'common.loading' : 'packages.emptyPlans') }}</p>
      </div>
    </div>
    <BaseDialog :show="form !== null" :title="t(form?.id ? 'packages.edit' : 'packages.create')" width="wide" @close="!saving && (form = null)">
      <form v-if="form" class="space-y-4" @submit.prevent="save">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1"><span class="input-label">{{ t('packages.name') }}</span><input v-model.trim="form.name" class="input" required maxlength="100" /></label>
          <label class="space-y-1"><span class="input-label">{{ t('packages.group') }}</span><select v-model.number="form.group_id" class="input" required><option :value="0" disabled>{{ t('keys.selectGroup') }}</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} ({{ group.platform }})</option></select></label>
          <label class="space-y-1"><span class="input-label">{{ t('packages.price') }}</span><input v-model.number="form.price" type="number" min="0.01" step="0.01" class="input" required /></label>
          <label class="space-y-1"><span class="input-label">{{ t('packages.currency') }}</span><input v-model.trim="form.currency" class="input uppercase" required pattern="[a-zA-Z]{3}" maxlength="3" /></label>
          <label class="space-y-1"><span class="input-label">{{ t('packages.cardType') }}</span><select v-model.number="form.validity_days" class="input" @change="setDefaultHours"><option :value="7">{{ t('packages.week') }} (7)</option><option :value="30">{{ t('packages.month') }} (7+7+7+9)</option></select></label>
          <label class="space-y-1"><span class="input-label">{{ t('packages.baseQuota') }} (USD)</span><input v-model.number="form.base_quota_usd" type="number" min="0.00000001" step="0.00000001" class="input" required /></label>
          <fieldset class="space-y-2 sm:col-span-2"><legend class="input-label">{{ t('packages.themeColor') }}</legend><div class="flex flex-wrap gap-2" role="radiogroup" :aria-label="t('packages.themeColor')"><label v-for="color in themeColors" :key="color" class="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-600"><input v-model="form.theme_color" class="peer sr-only" type="radio" name="package-theme-color" :value="color" /><span class="h-4 w-4 rounded-full ring-transparent peer-checked:ring-2 peer-checked:ring-gray-900 peer-checked:ring-offset-1 dark:peer-checked:ring-white" :style="{ backgroundColor: themeSpec(color, form.id).accent }" aria-hidden="true"></span><span>{{ t(`packages.themeColors.${color}`) }}</span></label></div></fieldset>
          <label class="space-y-1 sm:col-span-2"><span class="input-label">{{ t('packages.description') }}</span><textarea v-model="form.description" class="input" rows="3" /></label>
          <label class="space-y-1"><span class="input-label">{{ t('packages.sortOrder') }}</span><input v-model.number="form.sort_order" class="input" type="number" step="1" required /></label>
          <label class="flex items-center gap-2"><input v-model="form.for_sale" type="checkbox" />{{ t('packages.forSale') }}</label>
        </div>
        <label class="flex items-center gap-2"><input v-model="form.group_buy_enabled" type="checkbox" />{{ t('packages.groupEnabled') }}</label>
        <template v-if="form.group_buy_enabled">
          <label class="block space-y-1"><span class="input-label">{{ t('packages.hours') }}</span><input v-model.number="form.group_buy_hours" class="input" type="number" min="1" max="167" step="1" required /></label>
          <div v-for="(tier, index) in form.tiers" :key="index" class="flex items-end gap-2">
            <label class="flex-1 space-y-1"><span class="input-label">{{ t('packages.tierMembers') }}</span><input v-model.number="tier.members" class="input" type="number" min="2" step="1" required /></label>
            <label class="flex-1 space-y-1"><span class="input-label">{{ t('packages.tierQuota') }}</span><input v-model.number="tier.quota_usd" class="input" type="number" min="0.00000001" step="0.00000001" required /></label>
            <button type="button" class="btn btn-secondary" :aria-label="t('packages.removeTier')" @click="form.tiers.splice(index, 1)">×</button>
          </div>
          <button type="button" class="btn btn-secondary" @click="form.tiers.push({ members: (form.tiers.at(-1)?.members ?? 1) + 1, quota_usd: form.tiers.at(-1)?.quota_usd ?? form.base_quota_usd })">{{ t('packages.addTier') }}</button>
        </template>
        <p v-if="formError" role="alert" class="text-sm text-red-600">{{ formError }}</p>
        <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" :disabled="saving" @click="form = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="saving">{{ t(saving ? 'common.processing' : 'common.save') }}</button></div>
      </form>
    </BaseDialog>
  </AppLayout>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { packagesAPI } from '@/api/packages'
import { getAll } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { PACKAGE_THEME_COLOR_IDS, PACKAGE_THEME_COLORS, resolvePackageThemeColor, type PackagePlan } from '@/types/packages'
import { formatPaymentAmount } from '@/components/payment/currency'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
const { t } = useI18n()
const app = useAppStore()
const plans = ref<PackagePlan[]>([])
const groups = ref<AdminGroup[]>([])
const form = ref<PackagePlan | null>(null)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const formError = ref('')
const themeColors = PACKAGE_THEME_COLOR_IDS
function themeSpec(themeColor: string | undefined, planID: number) {
  const id = resolvePackageThemeColor(themeColor, planID)
  return { id, ...PACKAGE_THEME_COLORS[id] }
}
async function load() {
  loading.value = true
  try { [plans.value, groups.value] = await Promise.all([packagesAPI.adminPlans(), getAll()]); error.value = '' }
  catch (err) { error.value = extractApiErrorMessage(err, t('packages.loadError')) }
  finally { loading.value = false }
}
function edit(plan?: PackagePlan) {
  formError.value = ''
  form.value = plan ? { ...plan, theme_color: resolvePackageThemeColor(plan.theme_color, plan.id), tiers: plan.tiers.map(tier => ({ ...tier })) } : {
    id: 0, name: '', description: '', group_id: 0, group_name: '', group_platform: '',
    price: 0, currency: 'CNY', validity_days: 7, base_quota_usd: 0,
    group_buy_enabled: false, group_buy_hours: 24, tiers: [], theme_color: 'violet', for_sale: false, sort_order: 0,
  }
}
function setDefaultHours() { if (form.value) form.value.group_buy_hours = form.value.validity_days === 7 ? 24 : 48 }
async function save() {
  const plan = form.value
  if (!plan || saving.value) return
  const quotas = [plan.base_quota_usd, ...(plan.group_buy_enabled ? plan.tiers.map(tier => tier.quota_usd) : [])]
  const periods = plan.validity_days === 30 ? 4 : 1
  const invalid = !plan.name.trim() || plan.group_id <= 0 || !Number.isFinite(plan.price) || plan.price <= 0
    || !/^[A-Z]{3}$/.test(plan.currency.toUpperCase())
    || quotas.some(quota => !Number.isFinite(quota) || quota <= 0 || Math.round(quota * 1e8) % periods !== 0)
    || (plan.group_buy_enabled && (!Number.isInteger(plan.group_buy_hours) || plan.group_buy_hours <= 0 || plan.group_buy_hours >= 168 || !plan.tiers.length
      || plan.tiers.some((tier, index) => !Number.isInteger(tier.members) || tier.members <= (plan.tiers[index - 1]?.members ?? 1) || tier.quota_usd < (plan.tiers[index - 1]?.quota_usd ?? plan.base_quota_usd))))
  if (invalid) { formError.value = t('packages.invalidPlan'); return }
  saving.value = true
  try {
    await packagesAPI.savePlan({ ...plan, currency: plan.currency.toUpperCase() })
    form.value = null
    app.showSuccess(t('packages.saveSuccess'))
    await load()
  } catch (err) { formError.value = extractApiErrorMessage(err, t('common.error')) }
  finally { saving.value = false }
}
async function unpublish(plan: PackagePlan) {
  saving.value = true
  try { await packagesAPI.unpublish(plan.id); await load() }
  catch (err) { app.showError(extractApiErrorMessage(err, t('common.error'))) }
  finally { saving.value = false }
}
onMounted(load)
</script>
