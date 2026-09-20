<template>
  <AppLayout>
    <div class="space-y-5">
      <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('invoices.management') }}</h1>
      <details class="card p-5">
        <summary class="cursor-pointer font-semibold">{{ t('invoices.config') }}</summary>
        <p class="mt-3 text-sm text-gray-500">{{ t('invoices.configHint') }}</p>
        <p v-if="configError" role="alert" class="mt-3 text-red-600">{{ configError }} <button class="underline" @click="loadConfig">{{ t('common.refresh') }}</button></p>
        <form v-if="configLoaded" class="mt-4 space-y-4" @submit.prevent="saveConfig">
          <label class="flex items-center gap-2 text-sm"><input v-model="config.enabled" type="checkbox" class="h-4 w-4" />{{ t('invoices.enabled') }}</label>
          <div class="grid gap-4 sm:grid-cols-2">
            <div><label for="invoice-item" class="input-label">{{ t('invoices.itemName') }}</label><input id="invoice-item" v-model="config.item_name" class="input" maxlength="200" :required="config.enabled || !!config.tiers.length" /></div>
            <div><label for="invoice-rate" class="input-label">{{ t('invoices.taxRate') }} (%)</label><input id="invoice-rate" v-model.number="config.tax_rate" type="number" min="0" max="100" step="0.01" class="input" required /></div>
          </div>
          <div v-for="(tier, index) in config.tiers" :key="index" class="grid items-end gap-3 rounded-xl border border-gray-200 p-4 dark:border-dark-600 sm:grid-cols-4">
            <div><label :for="`tier-upper-${index}`" class="input-label">{{ t('invoices.tierTo') }}</label><input :id="`tier-upper-${index}`" :value="tier.upper_amount ?? ''" type="number" min="0.01" step="0.01" :required="index < config.tiers.length - 1" :placeholder="index === config.tiers.length - 1 ? t('invoices.noUpper') : ''" class="input" @input="tier.upper_amount = upperAmount($event)" /><p class="mt-1 text-xs text-gray-500">{{ t('invoices.tierFrom') }}: {{ index === 0 ? 0 : config.tiers[index - 1].upper_amount ?? '—' }}</p></div>
            <div><label :for="`tier-type-${index}`" class="input-label">{{ t('invoices.feeType') }}</label><select :id="`tier-type-${index}`" v-model="tier.type" class="input"><option value="fixed">{{ t('invoices.fixed') }}</option><option value="percentage">{{ t('invoices.percentage') }}</option></select></div>
            <div><label :for="`tier-value-${index}`" class="input-label">{{ t('invoices.feeValue') }}</label><input :id="`tier-value-${index}`" v-model.number="tier.value" type="number" min="0.01" :max="tier.type === 'percentage' ? 100 : undefined" step="0.01" class="input" required /></div>
            <button type="button" class="btn btn-secondary" @click="config.tiers.splice(index, 1)">{{ t('invoices.remove') }}</button>
          </div>
          <p class="text-xs text-gray-500">{{ t('invoices.lastTierHint') }}</p>
          <div class="flex flex-wrap gap-2"><button type="button" class="btn btn-secondary" @click="config.tiers.push({ upper_amount: null, type: 'fixed', value: 0 })">{{ t('invoices.addTier') }}</button><button class="btn btn-primary" :disabled="saving">{{ t(saving ? 'common.processing' : 'common.save') }}</button></div>
        </form>
      </details>
      <form class="card flex flex-wrap items-end gap-3 p-4" @submit.prevent="page = 1; loadInvoices()">
        <div class="min-w-48 flex-1"><label for="invoice-search" class="input-label">{{ t('invoices.search') }}</label><input id="invoice-search" v-model="filters.search" class="input" /></div>
        <div><label for="invoice-status" class="input-label">{{ t('invoices.invoice') }}</label><select id="invoice-status" v-model="filters.status" class="input"><option value="">{{ t('common.all') }}</option><option value="pending">{{ t('invoices.pending') }}</option><option value="issued">{{ t('invoices.issued') }}</option></select></div>
        <div class="w-28"><label for="invoice-user" class="input-label">{{ t('invoices.userId') }}</label><input id="invoice-user" v-model="filters.user_id" type="number" min="1" class="input" /></div>
        <div><label for="invoice-start" class="input-label">{{ t('invoices.startDate') }}</label><input id="invoice-start" v-model="filters.start_date" type="date" class="input" /></div>
        <div><label for="invoice-end" class="input-label">{{ t('invoices.endDate') }}</label><input id="invoice-end" v-model="filters.end_date" type="date" class="input" /></div>
        <button class="btn btn-secondary" :disabled="loading">{{ t('common.search') }}</button>
      </form>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-gray-500">{{ t('invoices.exportHint') }}</p>
        <div class="flex flex-wrap gap-2"><button class="btn btn-secondary" :disabled="exporting || !selected.length" @click="exportInvoices(false)">{{ t('invoices.exportSelected') }}</button><button class="btn btn-secondary" :disabled="exporting || !total" @click="exportInvoices(true)">{{ t('invoices.exportFiltered') }}</button><button class="btn btn-primary" :disabled="marking || !selected.length" @click="confirmMark = true">{{ t('invoices.markIssued') }}</button></div>
      </div>
      <p v-if="listError" role="alert" class="text-sm text-red-600">{{ listError }} <button class="underline" @click="loadInvoices">{{ t('common.refresh') }}</button></p>
      <DataTable :columns="columns" :data="invoices" :loading="loading" selectable :selected-keys="selected.map(invoice => invoice.id)" :selection-label="t('invoices.select')" @update:selected-keys="selectRows">
        <template #cell-id="{ row }"><span class="font-mono">#{{ row.id }}</span></template>
        <template #cell-title="{ row }"><p>{{ row.title }}</p><p class="text-xs text-gray-500">{{ row.tax_id }}</p></template>
        <template #cell-total_amount="{ row }"><strong>¥{{ row.quote.total_amount.toFixed(2) }}</strong><p class="text-xs text-gray-500">{{ t('invoices.serviceFee') }} ¥{{ row.quote.service_fee.toFixed(2) }}</p></template>
        <template #cell-status="{ row }"><span class="badge" :class="row.status === 'issued' ? 'badge-success' : 'badge-info'">{{ t(`invoices.status.${row.status}`) }}</span></template>
        <template #cell-submitted_at="{ row }">{{ formatDateTimeToMinute(row.submitted_at) }}</template>
        <template #cell-actions="{ row }"><button class="btn btn-secondary btn-sm" @click="detail = row">{{ t('common.view') }}</button></template>
      </DataTable>
      <Pagination v-if="total" :page="page" :page-size="pageSize" :total="total" @update:page="page = $event; loadInvoices()" @update:page-size="pageSize = $event; page = 1; loadInvoices()" />
      <BaseDialog :show="!!detail" :title="t('invoices.details')" @close="detail = null"><template v-if="detail"><InvoicePreview :quote="detail.quote" :information="detail" /><dl class="mt-4 space-y-2 break-words text-sm"><div>{{ t('invoices.email') }}: {{ detail.email }}</div><div>{{ t('invoices.remarks') }}: {{ detail.remarks || '—' }}</div><div v-if="detail.issued_at">{{ t('invoices.issuedAt') }}: {{ formatDateTimeToMinute(detail.issued_at) }} · {{ t('invoices.issuedBy') }}: #{{ detail.issued_by }}</div></dl></template></BaseDialog>
      <BaseDialog :show="confirmMark" :title="t('invoices.markIssued')" :show-close-button="!marking" :close-on-escape="!marking" @close="!marking && (confirmMark = false)"><p>{{ t('invoices.markConfirm', { count: selected.length }) }}</p><template #footer><button class="btn btn-secondary" :disabled="marking" @click="confirmMark = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="marking" @click="markIssued">{{ t('common.confirm') }}</button></template></BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { adminInvoiceAPI } from '@/api/invoices'
import type { InvoiceConfig, InvoiceListParams, InvoiceRequest } from '@/types/invoice'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import InvoicePreview from '@/components/payment/InvoicePreview.vue'
import { createInvoiceWorkbook } from './invoiceExport'

const { t } = useI18n()
const app = useAppStore()
const config = ref<InvoiceConfig>({ enabled: false, item_name: '', tax_rate: 0, tiers: [] })
const configLoaded = ref(false), configError = ref(''), saving = ref(false)
const invoices = ref<InvoiceRequest[]>([]), selected = ref<InvoiceRequest[]>([]), detail = ref<InvoiceRequest | null>(null)
const loading = ref(false), listError = ref(''), exporting = ref(false), marking = ref(false), confirmMark = ref(false)
const page = ref(1), pageSize = ref(20), total = ref(0)
const filters = reactive({ status: 'pending', search: '', user_id: '', start_date: '', end_date: '' })
const columns = computed<Column[]>(() => [
  { key: 'id', label: t('invoices.applicationId') }, { key: 'user_id', label: t('invoices.userId') }, { key: 'title', label: t('invoices.buyerTitle') },
  { key: 'total_amount', label: t('invoices.totalAmount') }, { key: 'status', label: t('invoices.invoice') }, { key: 'submitted_at', label: t('invoices.submittedAt') }, { key: 'actions', label: t('common.actions') },
])
function upperAmount(event: Event) { const raw = (event.target as HTMLInputElement).value; return raw === '' ? null : Number(raw) }
async function loadConfig() {
  configError.value = ''
  try { config.value = await adminInvoiceAPI.config(); configLoaded.value = true }
  catch (err) { configError.value = extractI18nErrorMessage(err, t, 'invoices.errors', t('invoices.loadFailed')) }
}
async function saveConfig() {
  if (saving.value || !configLoaded.value) return
  saving.value = true
  try { config.value = await adminInvoiceAPI.saveConfig(config.value); app.showSuccess(t('invoices.configSaved')); configError.value = '' }
  catch (err) { configError.value = extractI18nErrorMessage(err, t, 'invoices.errors', t('invoices.configurationInvalid')) }
  finally { saving.value = false }
}
function params(): InvoiceListParams { return { ...filters, user_id: Number(filters.user_id) || undefined } }
async function loadInvoices() {
  if (loading.value) return
  loading.value = true; listError.value = ''
  try { const result = await adminInvoiceAPI.list({ ...params(), page: page.value, page_size: pageSize.value }); invoices.value = result.items; total.value = result.total }
  catch (err) { listError.value = extractI18nErrorMessage(err, t, 'invoices.errors', t('invoices.loadFailed')) }
  finally { loading.value = false }
}
function selectRows(keys: Array<string | number>) {
  const wanted = new Set(keys.map(Number))
  const records = new Map([...selected.value, ...invoices.value].map(invoice => [invoice.id, invoice]))
  selected.value = [...records.values()].filter(invoice => wanted.has(invoice.id))
}
async function markIssued() {
  if (marking.value || !selected.value.length) return
  marking.value = true
  try { await adminInvoiceAPI.markIssued(selected.value.map(invoice => invoice.id)); selected.value = []; confirmMark.value = false; app.showSuccess(t('invoices.marked')); await loadInvoices() }
  catch (err) { app.showError(extractI18nErrorMessage(err, t, 'invoices.errors', t('common.error'))) }
  finally { marking.value = false }
}
async function exportInvoices(filtered: boolean) {
  if (exporting.value) return
  exporting.value = true
  try {
    let rows = [...selected.value]
    if (filtered) {
      rows = []
      const filter = params()
      for (let current = 1; ; current++) {
        const result = await adminInvoiceAPI.list({ ...filter, page: current, page_size: 100 })
        rows.push(...result.items)
        if (rows.length >= result.total || result.items.length < 100) break
      }
    }
    const { workbook, XLSX } = await createInvoiceWorkbook(rows, t)
    saveAs(new Blob([XLSX.write(workbook, { bookType: 'xlsx', type: 'array' })], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }), `invoices-${new Date().toISOString().slice(0, 10)}.xlsx`)
  } catch (err) { app.showError(extractI18nErrorMessage(err, t, 'invoices.errors', t('invoices.exportFailed'))) }
  finally { exporting.value = false }
}
onMounted(() => { void loadConfig(); void loadInvoices() })
</script>
