<template>
  <DataTable :columns="columns" :data="orders" :loading="loading">
    <template #cell-invoice_selection="{ row }"><input type="checkbox" class="h-4 w-4 rounded border-gray-300" :checked="selectedInvoiceIds?.includes(row.id)" :disabled="!canInvoiceOrder(row)" :aria-label="t('invoices.select') + ' #' + row.id" @change="$emit('toggleInvoice', row.id)" /></template>
    <template #cell-order_type="{ row }"><span class="text-sm">{{ t(`invoices.orderTypes.${row.order_type}`) }}</span><p v-if="row.order_type === 'invoice_fee' && row.invoice" class="mt-1 text-xs text-gray-500">{{ t('invoices.feeInvoiceTotal', { amount: row.invoice.total_amount.toFixed(2) }) }}</p></template>
    <template #cell-invoice="{ row }"><slot name="invoice" :row="row"><span v-if="row.invoice" class="badge badge-info">{{ t(`invoices.status.${row.invoice.status}`) }}</span><span v-else class="text-xs text-gray-400">—</span></slot></template>
    <template #cell-id="{ value }">
      <span class="font-mono text-sm">#{{ value }}</span>
    </template>
    <template #cell-out_trade_no="{ value }">
      <span class="text-sm text-gray-900 dark:text-white">{{ value }}</span>
    </template>
    <template v-if="showUser" #cell-user_email="{ value, row }">
      <div class="text-sm">
        <span class="text-gray-900 dark:text-white">{{ value || row.user_name || '#' + row.user_id }}</span>
        <span v-if="row.user_notes" class="ml-1 text-xs text-gray-400">({{ row.user_notes }})</span>
      </div>
    </template>
    <template #cell-pay_amount="{ value, row }">
      <div class="text-sm">
        <span class="font-medium text-gray-900 dark:text-white">{{ paymentAmountSymbol(row) }}{{ value.toFixed(2) }}</span>
        <div v-if="row.discount" class="text-xs text-emerald-600">{{ row.discount.code }} · {{ t('payment.coupon.discount') }} {{ row.discount.discount_amount.toFixed(2) }} {{ t('payment.coupon.priceUnits') }}</div>
        <span v-if="row.fee_rate > 0" class="ml-1 text-xs text-gray-400" :title="t('payment.orders.fee') + ': ' + row.fee_rate + '%'">
          ({{ t('payment.orders.fee') }} {{ row.fee_rate }}%)
        </span>
        <div v-if="row.order_type === 'balance' && row.amount !== row.pay_amount" class="text-xs text-gray-500">
          {{ t('payment.orders.creditedAmount') }}: {{ creditedAmountSymbol }}{{ row.amount.toFixed(2) }}
        </div>
      </div>
    </template>
    <template #cell-payment_type="{ value }">
      <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('payment.methods.' + value, value) }}</span>
    </template>
    <template #cell-status="{ value }">
      <OrderStatusBadge :status="value" />
    </template>
    <template #cell-created_at="{ value }">
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDate(value) }}</span>
    </template>
    <template #cell-actions="{ row }">
      <slot name="actions" :row="row" />
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PaymentOrder } from '@/types/payment'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import { currencySymbol } from '@/components/payment/currency'
import { canInvoiceOrder } from './invoiceEligibility'

const { t } = useI18n()

const props = defineProps<{
  orders: PaymentOrder[]
  loading: boolean
  showUser?: boolean
  showInvoices?: boolean
  selectInvoices?: boolean
  selectedInvoiceIds?: number[]
}>()
defineEmits<{ toggleInvoice: [id: number] }>()

function formatDate(dateStr: string) { return new Date(dateStr).toLocaleString() }

const creditedAmountSymbol = currencySymbol('USD')

function paymentAmountSymbol(order: PaymentOrder): string {
  return currencySymbol(order.currency)
}

const columns = computed((): Column[] => {
  const cols: Column[] = [
    { key: 'id', label: t('payment.orders.orderId') },
    { key: 'out_trade_no', label: t('payment.orders.orderNo') },
  ]
  if (props.showUser) {
    cols.push({ key: 'user_email', label: t('payment.admin.colUser') })
  }
  if (props.selectInvoices) cols.unshift({ key: 'invoice_selection', label: t('invoices.select') })
  cols.push({ key: 'order_type', label: t('payment.orders.orderType') })
  if (props.showInvoices) cols.push({ key: 'invoice', label: t('invoices.invoice') })
  cols.push(
    { key: 'pay_amount', label: t('payment.orders.payAmount') },
    { key: 'payment_type', label: t('payment.orders.paymentMethod') },
    { key: 'status', label: t('payment.orders.status') },
    { key: 'created_at', label: t('payment.orders.createdAt') },
    { key: 'actions', label: t('common.actions') },
  )
  return cols
})
</script>
