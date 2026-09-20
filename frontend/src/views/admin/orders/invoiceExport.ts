import type { InvoiceRequest } from '@/types/invoice'

export async function createInvoiceWorkbook(invoices: InvoiceRequest[], t: (key: string) => string) {
  const XLSX = await import('xlsx')
  const summary = XLSX.utils.aoa_to_sheet([
    ['applicationId', 'userId', 'buyerTitle', 'taxId', 'email', 'baseAmount', 'serviceFee', 'totalAmount', 'netAmount', 'taxRate', 'taxAmount', 'itemName', 'remarks', 'submittedAt', 'issuedAt', 'issuedBy', 'invoice'].map(key => t(`invoices.${key}`)),
    ...invoices.map(invoice => [String(invoice.id), String(invoice.user_id), invoice.title, invoice.tax_id, invoice.email,
      invoice.quote.base_amount, invoice.quote.service_fee, invoice.quote.total_amount, invoice.quote.net_amount, invoice.quote.tax_rate, invoice.quote.tax_amount,
      invoice.quote.item_name, invoice.remarks, invoice.submitted_at ?? '', invoice.issued_at ?? '', invoice.issued_by == null ? '' : String(invoice.issued_by), t(`invoices.status.${invoice.status}`)]),
  ])
  const orders = XLSX.utils.aoa_to_sheet([
    [t('invoices.applicationId'), t('payment.orders.orderId'), t('payment.orders.orderNo'), t('invoices.itemName'), t('invoices.baseAmount')],
    ...invoices.flatMap(invoice => invoice.quote.orders.map(order => [String(invoice.id), String(order.id), order.order_no, order.name || t(`invoices.orderTypes.${order.order_type}`), order.amount])),
  ])
  // String cells preserve identifiers and prevent buyer-controlled text becoming formulas.
  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, summary, t('invoices.invoiceSheet'))
  XLSX.utils.book_append_sheet(workbook, orders, t('invoices.ordersSheet'))
  return { workbook, XLSX }
}
