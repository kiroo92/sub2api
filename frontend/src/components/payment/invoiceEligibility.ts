import type { PaymentOrder } from '@/types/payment'
import { normalizePaymentCurrency } from './currency'

export function canInvoiceOrder(order: PaymentOrder): boolean {
  return (order.order_type === 'balance' || order.order_type === 'subscription')
    && order.status === 'COMPLETED' && !!order.paid_at && !order.invoice
    && order.refund_amount === 0 && order.pay_amount > 0 && normalizePaymentCurrency(order.currency) === 'CNY'
}
