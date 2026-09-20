<template>
  <div class="space-y-5">
    <header class="flex flex-wrap items-center gap-4"><router-link to="/orders" class="text-sm text-primary-600">← {{ t('invoices.back') }}</router-link><h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('invoices.title') }}</h1></header>
    <p v-if="loading" class="py-8 text-center text-gray-500">{{ t('common.loading') }}</p>
    <div v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }} <button class="ml-2 underline" :disabled="loading || busy" @click="load()">{{ t('common.refresh') }}</button></div>
    <section v-if="unpaid.length" class="card space-y-4 p-5" data-unpaid-invoices>
      <p role="status" class="text-sm font-medium text-amber-700 dark:text-amber-300">{{ t('invoices.unpaidNotice') }}</p>
      <div v-for="invoice in unpaid" :key="invoice.id" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-gray-200 p-4 dark:border-dark-600">
        <div class="text-sm"><p class="font-medium">{{ t('invoices.applicationId') }} #{{ invoice.id }} · ¥{{ invoice.total_amount.toFixed(2) }}</p><p class="mt-1 text-xs text-gray-500">{{ t('invoices.serviceFee') }} ¥{{ invoice.service_fee.toFixed(2) }} · {{ formatDateTimeToMinute(invoice.created_at) }}</p></div>
        <button class="btn btn-secondary btn-sm" :disabled="cancelling || busy" @click="cancelTarget = invoice.id">{{ t('invoices.cancelApplication') }}</button>
      </div>
      <button class="text-sm text-primary-600 underline" :disabled="loading || cancelling" @click="load()">{{ t('common.refresh') }}</button>
    </section>
    <section v-else-if="cancelled" class="card space-y-4 p-5" data-cancelled-invoice><p role="status">{{ t('invoices.cancelledHint') }}</p><button class="btn btn-primary" :disabled="loading" @click="startAgain">{{ t('invoices.applyAgain') }}</button></section>
    <section v-else-if="empty" role="status" class="rounded-xl border border-dashed border-gray-300 p-8 text-center text-sm text-gray-500 dark:border-dark-600">{{ t('invoices.noOrders') }}</section>
    <template v-else-if="quote">
      <div class="rounded-2xl border border-primary-200 bg-primary-50/60 p-5 dark:border-primary-900 dark:bg-primary-900/10">
        <dl class="space-y-3 text-sm">
          <div class="flex justify-between gap-3"><dt class="text-gray-500">{{ t('invoices.baseAmount') }}</dt><dd>¥{{ quote.base_amount.toFixed(2) }}</dd></div>
          <div class="flex justify-between gap-3"><dt class="text-gray-500">{{ t('invoices.serviceFee') }}</dt><dd class="font-medium text-primary-600">¥{{ quote.service_fee.toFixed(2) }}</dd></div>
          <div class="flex justify-between gap-3 border-t border-primary-200 pt-3 dark:border-primary-900"><dt>{{ t('invoices.totalAmount') }}</dt><dd class="text-xl font-semibold">¥{{ quote.total_amount.toFixed(2) }}</dd></div>
        </dl>
        <p class="mt-3 text-xs text-gray-500">{{ t('invoices.totalHint') }}</p>
      </div>
      <div class="card p-5">
        <h2 class="mb-1 font-semibold">{{ t('invoices.selectedOrders', { count: quote.orders.length }) }}</h2><p class="mb-4 text-xs text-gray-500">{{ t('invoices.wholeOrders') }}</p>
        <div class="max-h-72 space-y-2 overflow-y-auto">
          <div v-for="order in quote.orders" :key="order.id" class="flex items-center justify-between gap-3 rounded-xl border border-gray-200 p-3 dark:border-dark-600">
            <div class="min-w-0"><p class="break-words text-sm font-medium">{{ order.name || t(`invoices.orderTypes.${order.order_type}`) }}</p><p class="mt-1 break-all text-xs text-gray-500">{{ order.order_no }}</p></div>
            <div class="flex shrink-0 items-center gap-3 text-sm"><span class="tabular-nums">¥{{ order.amount.toFixed(2) }}</span><button v-if="!saved" :disabled="loading || busy || preparing" class="text-gray-500 hover:text-red-500" @click="remove(order.id)">{{ t('invoices.remove') }}</button></div>
          </div>
        </div>
      </div>
      <form ref="informationForm" class="card space-y-4 p-5" @submit.prevent="preview">
        <h2 class="font-semibold">{{ t('invoices.information') }}</h2>
        <div><label for="invoice-tax" class="input-label">{{ t('invoices.taxId') }}</label><input id="invoice-tax" v-model="information.tax_id" class="input" required maxlength="64" :disabled="!!saved || preparing || busy" /></div>
        <div><label for="invoice-title" class="input-label">{{ t('invoices.buyerTitle') }}</label><input id="invoice-title" v-model="information.title" class="input" required maxlength="200" :disabled="!!saved || preparing || busy" /></div>
        <div><label for="invoice-email" class="input-label">{{ t('invoices.email') }}</label><input id="invoice-email" v-model="information.email" class="input" type="email" required maxlength="254" :disabled="!!saved || preparing || busy" /></div>
        <div><label for="invoice-remarks" class="input-label">{{ t('invoices.remarks') }}</label><textarea id="invoice-remarks" v-model="information.remarks" class="input" rows="3" maxlength="1000" :disabled="!!saved || preparing || busy" /></div>
        <button type="submit" class="btn btn-secondary" :disabled="preparing || busy">{{ t('invoices.preview') }}</button>
      </form>
      <p v-if="saved && saved.status !== 'awaiting_payment'" class="card p-5 text-sm"><strong>{{ t(`invoices.status.${saved.status}`) }}</strong><span v-if="saved.status !== 'cancelled'" class="ml-2 text-gray-500">{{ t('invoices.pendingHint') }}</span></p>
      <slot v-else name="payment" :quote="quote" :preparing="preparing" />
    </template>
    <BaseDialog :show="showPreview" :title="t('invoices.previewTitle')" @close="showPreview = false"><InvoicePreview v-if="quote" :quote="quote" :information="information" /></BaseDialog>
    <BaseDialog :show="cancelTarget !== null" :title="t('invoices.cancelApplication')" :show-close-button="!cancelling" :close-on-escape="!cancelling" @close="!cancelling && (cancelTarget = null)"><p class="text-sm">{{ t('invoices.cancelConfirm') }}</p><template #footer><button class="btn btn-secondary" :disabled="cancelling" @click="cancelTarget = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="cancelling" @click="cancelApplication">{{ t(cancelling ? 'common.processing' : 'common.confirm') }}</button></template></BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { invoiceAPI } from '@/api/invoices'
import { extractApiErrorCode, extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import type { CreateInvoiceRequest, InvoiceInformation, InvoiceQuote, InvoiceRequest, UnpaidInvoice } from '@/types/invoice'
import BaseDialog from '@/components/common/BaseDialog.vue'
import InvoicePreview from './InvoicePreview.vue'
import { PAYMENT_RECOVERY_STORAGE_KEY, clearPaymentRecoverySnapshot, readPaymentRecoverySnapshot } from './paymentFlow'

defineProps<{ busy?: boolean }>()
const emit = defineEmits<{ quote: [quote: InvoiceQuote | null]; loaded: [invoice: InvoiceRequest] }>()
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const quote = ref<InvoiceQuote | null>(null)
const saved = ref<InvoiceRequest | null>(null)
const information = reactive<InvoiceInformation>({ tax_id: '', title: '', email: '', remarks: '' })
const informationForm = ref<HTMLFormElement | null>(null)
const loading = ref(false)
const preparing = ref(false)
const error = ref('')
const unpaid = ref<UnpaidInvoice[]>([])
const cancelTarget = ref<number | null>(null)
const cancelling = ref(false)
const cancelled = ref(false)
const empty = ref(false)
const showPreview = ref(false)
let requestKey = ''
let requestPayload = ''
let selectedIds = typeof route.query.ids === 'string' ? route.query.ids.split(',').map(Number) : []
let selectAll = route.query.selection === 'all'

async function load(ids?: number[]) {
  if (loading.value) return
  loading.value = true; error.value = ''; quote.value = null; empty.value = false; emit('quote', null)
  try {
    const invoiceId = Number(route.query.invoice_request_id)
    if (invoiceId > 0) {
      saved.value = await invoiceAPI.get(invoiceId)
      if (saved.value.status === 'awaiting_payment') { unpaid.value = await invoiceAPI.unpaid(); return }
      unpaid.value = []
      Object.assign(information, { tax_id: saved.value.tax_id, title: saved.value.title, email: saved.value.email, remarks: saved.value.remarks })
      quote.value = saved.value.quote
      emit('loaded', saved.value)
    } else {
      unpaid.value = await invoiceAPI.unpaid()
      if (unpaid.value.length || cancelled.value) return
      if (ids) { selectedIds = ids; selectAll = false }
      quote.value = await invoiceAPI.quote(selectAll ? { selection: 'all' } : { selection: 'selected', order_ids: selectedIds })
      selectedIds = quote.value.orders.map(order => order.id)
      selectAll = false
    }
    emit('quote', quote.value)
  } catch (err) { await showInvoiceError(err, 'invoices.loadFailed') }
  finally { loading.value = false }
}
async function remove(id: number) {
  const ids = quote.value?.orders.filter(order => order.id !== id).map(order => order.id) ?? []
  if (!ids.length) { selectedIds = []; selectAll = false; quote.value = null; emit('quote', null); empty.value = true; return }
  await load(ids)
}
function validInformation() {
  if (!informationForm.value?.reportValidity() || !information.tax_id.trim() || !information.title.trim()) { error.value = t('invoices.required'); return false }
  return true
}
function preview() { if (saved.value || validInformation()) showPreview.value = true }
async function prepare(): Promise<InvoiceRequest | null> {
  if (unpaid.value.length || cancelling.value || cancelled.value) return null
  if (saved.value) return saved.value
  if (!quote.value || preparing.value || loading.value || !validInformation()) return null
  const payload: CreateInvoiceRequest = { ...information, order_ids: quote.value.orders.map(order => order.id), quote_fingerprint: quote.value.fingerprint }
  const serialized = JSON.stringify(payload)
  // Keep the key for an uncertain retry; editing the form creates a different operation.
  if (!requestKey || serialized !== requestPayload) { requestKey = crypto.randomUUID(); requestPayload = serialized }
  preparing.value = true; error.value = ''
  try {
    saved.value = await invoiceAPI.create(payload, requestKey)
    await router.replace({ path: '/orders/invoice', query: { invoice_request_id: String(saved.value.id) } })
    emit('loaded', saved.value)
    return saved.value
  } catch (err) { await showInvoiceError(err, 'invoices.applyFailed'); return null }
  finally { preparing.value = false }
}
async function showInvoiceError(err: unknown, fallback: string) {
  const code = extractApiErrorCode(err)
  if (code === 'INVOICE_NO_ORDERS') { empty.value = true; return }
  error.value = code?.startsWith('INVOICE_') ? extractI18nErrorMessage(err, t, 'invoices.errors', t(fallback)) : t(fallback)
  if (code === 'INVOICE_UNPAID_EXISTS') {
    try { unpaid.value = await invoiceAPI.unpaid(); if (unpaid.value.length) error.value = '' } catch { /* Keep the original localized error. */ }
  }
}
async function cancelApplication() {
  if (cancelTarget.value === null || cancelling.value) return
  cancelling.value = true; error.value = ''
  const id = cancelTarget.value
  try {
    await invoiceAPI.cancel(id)
    const recovery = readPaymentRecoverySnapshot(localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY))
    if (recovery?.orderType === 'invoice_fee' && recovery.invoiceRequestId === id) clearPaymentRecoverySnapshot(localStorage)
    unpaid.value = unpaid.value.filter(invoice => invoice.id !== id)
    cancelled.value = true
    saved.value = null; quote.value = null; emit('quote', null)
    unpaid.value = await invoiceAPI.unpaid()
  } catch (err) { await showInvoiceError(err, 'invoices.cancelFailed') }
  finally { cancelling.value = false; cancelTarget.value = null }
}
async function startAgain() {
  if (loading.value || unpaid.value.length) return
  await router.replace({ path: '/orders/invoice', query: { selection: 'all' } })
  saved.value = null; cancelled.value = false; selectedIds = []; selectAll = true; requestKey = ''; requestPayload = ''
  await load()
}
defineExpose({ prepare, reload: load })
onMounted(() => { void load() })
</script>
