<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h1 class="text-xl font-semibold">{{ t('payment.coupon.management') }}</h1>
        <button class="btn btn-primary" data-testid="create-discount" @click="edit()">{{ t('payment.coupon.create') }}</button>
      </div>
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.coupon.adminHint') }}</p>
      <form class="flex gap-2" @submit.prevent="page = 1; load()">
        <input v-model="search" class="input max-w-xs" :aria-label="t('payment.coupon.label')" :placeholder="t('payment.coupon.placeholder')" />
        <button class="btn btn-secondary" :disabled="loading">{{ t('common.search') }}</button>
      </form>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <div class="card overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead><tr class="border-b border-gray-200 dark:border-dark-700">
            <th class="p-3">{{ t('payment.coupon.label') }}</th><th class="p-3">{{ t('payment.coupon.discount') }}</th>
            <th class="p-3">{{ t('payment.coupon.scope') }}</th><th class="p-3">{{ t('payment.coupon.usage') }}</th>
            <th class="p-3">{{ t('payment.coupon.expiry') }}</th><th class="p-3">{{ t('common.status') }}</th><th class="p-3">{{ t('common.actions') }}</th>
          </tr></thead>
          <tbody>
            <tr v-for="code in codes" :key="code.id" class="border-b border-gray-100 dark:border-dark-700">
              <td class="p-3 font-mono">{{ code.code }}</td>
              <td class="p-3">{{ code.discount_type === 'percentage' ? t('payment.coupon.percentSummary', { value: code.discount_value }) : t('payment.coupon.fixedSummary', { value: code.discount_value }) }}</td>
              <td class="p-3">{{ code.plan_ids?.length ? code.plan_ids.map(id => plans.find(p => p.id === id)?.name || `#${id}`).join(', ') : t('payment.coupon.allPlans') }}</td>
              <td class="p-3">
                <div>{{ t('payment.coupon.usageSummary', { used: code.used_count, reserved: code.reserved_count, max: code.max_uses || '∞' }) }}</div>
                <div class="text-xs text-gray-500">{{ t('payment.coupon.perUserSummary', { max: code.per_user_limit || '∞' }) }}</div>
              </td>
              <td class="p-3">{{ code.expires_at ? new Date(code.expires_at).toLocaleString() : '—' }}</td>
              <td class="p-3">{{ t(code.enabled ? 'payment.coupon.enabled' : 'payment.coupon.disabled') }}</td>
              <td class="p-3"><div class="flex gap-3">
                <button class="text-primary-600" @click="edit(code)">{{ t('common.edit') }}</button>
                <RouterLink class="text-primary-600" :to="{ path: '/admin/orders', query: { discount_code_id: code.id } }">{{ t('payment.coupon.orders') }}</RouterLink>
              </div></td>
            </tr>
            <tr v-if="!codes.length"><td colspan="7" class="p-8 text-center text-gray-500">{{ loading ? t('common.loading') : t('payment.coupon.empty') }}</td></tr>
          </tbody>
        </table>
      </div>
      <div class="flex items-center justify-end gap-3">
        <button class="btn btn-secondary" :disabled="loading || page <= 1" @click="page--; load()">{{ t('payment.coupon.previous') }}</button>
        <span>{{ page }} / {{ Math.max(1, Math.ceil(total / 20)) }}</span>
        <button class="btn btn-secondary" :disabled="loading || page * 20 >= total" @click="page++; load()">{{ t('payment.coupon.next') }}</button>
      </div>
    </div>
    <BaseDialog :show="show" :title="t(editingId ? 'payment.coupon.edit' : 'payment.coupon.create')" @close="!saving && (show = false)">
      <form id="discount-form" class="space-y-4" @submit.prevent="save">
        <div><label class="input-label" for="discount-code">{{ t('payment.coupon.label') }}</label><input id="discount-code" v-model="form.code" class="input" required pattern="[A-Za-z0-9_-]+" maxlength="64" :disabled="!!editingId || saving" /></div>
        <div class="grid grid-cols-2 gap-3">
          <div><label class="input-label" for="discount-type">{{ t('payment.coupon.type') }}</label><select id="discount-type" v-model="form.discount_type" class="input"><option value="percentage">{{ t('payment.coupon.percentage') }}</option><option value="fixed_amount">{{ t('payment.coupon.fixed') }}</option></select></div>
          <div><label class="input-label" for="discount-value">{{ t(form.discount_type === 'percentage' ? 'payment.coupon.fold' : 'payment.coupon.amount') }}</label><input id="discount-value" v-model.number="displayValue" class="input" type="number" :min="form.discount_type === 'percentage' ? 0.001 : 0.01" :max="form.discount_type === 'percentage' ? 9.999 : undefined" :step="form.discount_type === 'percentage' ? 0.001 : 0.01" required /></div>
        </div>
        <p class="text-xs text-gray-500">{{ t('payment.coupon.valueHint') }}</p>
        <fieldset class="space-y-2"><legend class="input-label">{{ t('payment.coupon.scope') }}</legend>
          <p class="text-xs text-gray-500">{{ t('payment.coupon.scopeHint') }}</p>
          <div class="max-h-40 space-y-2 overflow-auto"><label v-for="plan in selectablePlans" :key="plan.id" class="flex items-center gap-2 text-sm"><input v-model="form.plan_ids" type="checkbox" :value="plan.id" />{{ plan.name }}</label></div>
        </fieldset>
        <div class="grid grid-cols-2 gap-3">
          <div><label class="input-label" for="discount-total">{{ t('payment.coupon.maxUses') }}</label><input id="discount-total" v-model.number="form.max_uses" class="input" type="number" min="0" step="1" required /></div>
          <div><label class="input-label" for="discount-user">{{ t('payment.coupon.perUser') }}</label><input id="discount-user" v-model.number="form.per_user_limit" class="input" type="number" min="0" step="1" required /></div>
        </div>
        <div><label class="input-label" for="discount-expiry">{{ t('payment.coupon.expiry') }}</label><input id="discount-expiry" v-model="expiry" class="input" type="datetime-local" /></div>
        <label class="flex items-center gap-2 text-sm"><input v-model="form.enabled" type="checkbox" />{{ t('payment.coupon.enabled') }}</label>
        <p v-if="saveError" role="alert" class="text-sm text-red-600">{{ saveError }}</p>
      </form>
      <template #footer><button class="btn btn-secondary" :disabled="saving" @click="show = false">{{ t('common.cancel') }}</button><button form="discount-form" type="submit" class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { DiscountCode, DiscountCodeInput, SubscriptionPlan } from '@/types/payment'

const { t } = useI18n()
const codes = ref<DiscountCode[]>([])
const plans = ref<SubscriptionPlan[]>([])
const page = ref(1)
const total = ref(0)
const search = ref('')
const loading = ref(false)
const error = ref('')
const show = ref(false)
const saving = ref(false)
const saveError = ref('')
const editingId = ref(0)
const expiry = ref('')
const form = reactive<DiscountCodeInput>({ code: '', discount_type: 'percentage', discount_value: 80, plan_ids: [], enabled: true, expires_at: null, max_uses: 0, per_user_limit: 1 })
const displayValue = computed({ get: () => form.discount_type === 'percentage' ? form.discount_value / 10 : form.discount_value, set: (value: number) => { form.discount_value = form.discount_type === 'percentage' ? Math.round(value * 1000) / 100 : value } })
const selectablePlans = computed(() => [...plans.value, ...form.plan_ids.filter(id => !plans.value.some(plan => plan.id === id)).map(id => ({ id, name: `#${id}` }))])

function edit(code?: DiscountCode) {
  editingId.value = code?.id || 0
  Object.assign(form, { code: code?.code || '', discount_type: code?.discount_type || 'percentage', discount_value: code?.discount_value ?? 80,
    plan_ids: [...(code?.plan_ids || [])], enabled: code ? !!code.enabled : true, expires_at: code?.expires_at || null,
    max_uses: code?.max_uses || 0, per_user_limit: code ? (code.per_user_limit || 0) : 1 })
  const date = code?.expires_at ? new Date(code.expires_at) : null
  expiry.value = date ? new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16) : ''
  saveError.value = ''
  show.value = true
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await adminPaymentAPI.getDiscountCodes({ page: page.value, page_size: 20, search: search.value })
    codes.value = data.items
    total.value = data.total
  } catch (err) { error.value = extractApiErrorMessage(err, t('common.error')) }
  finally { loading.value = false }
}

async function save() {
  if (saving.value) return
  saving.value = true
  saveError.value = ''
  try {
    const data = { ...form, code: form.code.trim().toUpperCase(), plan_ids: [...form.plan_ids], expires_at: expiry.value ? new Date(expiry.value).toISOString() : null }
    if (editingId.value) await adminPaymentAPI.updateDiscountCode(editingId.value, data)
    else await adminPaymentAPI.createDiscountCode(data)
    show.value = false
    await load()
  } catch (err) { saveError.value = extractApiErrorMessage(err, t('common.error')) }
  finally { saving.value = false }
}

onMounted(async () => {
  await load()
  try { plans.value = (await adminPaymentAPI.getPlans()).data }
  catch (err) { error.value = extractApiErrorMessage(err, t('common.error')) }
})
</script>
