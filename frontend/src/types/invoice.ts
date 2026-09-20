import type { CreateOrderResult } from './payment'

export type InvoiceStatus = 'awaiting_payment' | 'pending' | 'issued' | 'cancelled'
export interface InvoiceFeeTier { upper_amount: number | null; type: 'fixed' | 'percentage'; value: number }
export interface InvoiceConfig { enabled: boolean; item_name: string; tax_rate: number; tiers: InvoiceFeeTier[] }
export interface InvoiceOrderLine { id: number; order_no: string; order_type: 'balance' | 'subscription'; name: string; amount: number }
export interface InvoiceQuote {
  orders: InvoiceOrderLine[]; currency: string; base_amount: number; service_fee: number; total_amount: number
  net_amount: number; tax_amount: number; item_name: string; tax_rate: number; tier: InvoiceFeeTier; fingerprint: string
}
export interface InvoiceInformation { tax_id: string; title: string; email: string; remarks: string }
export interface CreateInvoiceRequest extends InvoiceInformation { order_ids: number[]; quote_fingerprint: string }
export interface InvoiceSummary { id: number; status: InvoiceStatus; total_amount: number }
export interface InvoiceRequest extends InvoiceInformation {
  id: number; user_id: number; status: InvoiceStatus; quote: InvoiceQuote; payment?: CreateOrderResult
  created_at: string; expires_at: string; submitted_at: string | null; issued_at: string | null; issued_by: number | null
}
export interface InvoiceListParams { page?: number; page_size?: number; status?: string; search?: string; user_id?: number; start_date?: string; end_date?: string }
