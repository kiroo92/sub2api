<template>
  <div class="space-y-4 text-sm text-gray-700 dark:text-gray-200">
    <h2 class="py-3 text-center text-xl font-semibold text-rose-600 dark:text-rose-400">{{ t('invoices.electronicInvoice') }}</h2>
    <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
      <h3 class="mb-3 border-b border-gray-200 pb-2 font-medium dark:border-dark-600">{{ t('invoices.buyer') }}</h3>
      <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2">
        <dt class="text-gray-500">{{ t('invoices.buyerTitle') }}</dt><dd class="break-words font-medium">{{ information.title }}</dd>
        <dt class="text-gray-500">{{ t('invoices.taxId') }}</dt><dd class="break-all font-mono">{{ information.tax_id }}</dd>
      </dl>
    </section>
    <section class="grid grid-cols-2 gap-4 rounded-xl border border-gray-200 p-4 dark:border-dark-600 sm:grid-cols-[2fr_1fr_1fr_1fr]">
      <div><p class="mb-2 text-xs text-gray-500">{{ t('invoices.itemName') }}</p><p class="break-words">{{ quote.item_name }}</p></div>
      <div><p class="mb-2 text-xs text-gray-500">{{ t('invoices.netAmount') }}</p><p class="tabular-nums">¥{{ quote.net_amount.toFixed(2) }}</p></div>
      <div><p class="mb-2 text-xs text-gray-500">{{ t('invoices.taxRate') }}</p><p>{{ quote.tax_rate }}%</p></div>
      <div><p class="mb-2 text-xs text-gray-500">{{ t('invoices.taxAmount') }}</p><p class="tabular-nums">¥{{ quote.tax_amount.toFixed(2) }}</p></div>
    </section>
    <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-gray-200 p-4 dark:border-dark-600"><span>{{ t('invoices.grossAmount') }}</span><strong class="text-xl tabular-nums">¥{{ quote.total_amount.toFixed(2) }}</strong></div>
    <p class="text-center text-xs text-gray-500">{{ t('invoices.previewHint') }}</p>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { InvoiceInformation, InvoiceQuote } from '@/types/invoice'
defineProps<{ information: InvoiceInformation; quote: InvoiceQuote }>()
const { t } = useI18n()
</script>
