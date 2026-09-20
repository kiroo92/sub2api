import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type { CreateInvoiceRequest, InvoiceConfig, InvoiceListParams, InvoiceQuote, InvoiceRequest, UnpaidInvoice } from '@/types/invoice'

export const invoiceAPI = {
  async unpaid() { return (await apiClient.get<UnpaidInvoice[]>('/payment/invoices/unpaid')).data },
  async cancel(id: number) { return (await apiClient.post<InvoiceRequest>(`/payment/invoices/${id}/cancel`)).data },
  async config() { return (await apiClient.get<InvoiceConfig>('/payment/invoices/config')).data },
  async quote(selection: { selection: 'all' | 'selected'; order_ids?: number[] }) {
    return (await apiClient.post<InvoiceQuote>('/payment/invoices/quote', selection)).data
  },
  async create(data: CreateInvoiceRequest, key: string) {
    return (await apiClient.post<InvoiceRequest>('/payment/invoices', data, { headers: { 'Idempotency-Key': key } })).data
  },
  async get(id: number) { return (await apiClient.get<InvoiceRequest>(`/payment/invoices/${id}`)).data },
}
export const adminInvoiceAPI = {
  async config() { return (await apiClient.get<InvoiceConfig>('/admin/payment/invoices/config')).data },
  async saveConfig(data: InvoiceConfig) { return (await apiClient.put<InvoiceConfig>('/admin/payment/invoices/config', data)).data },
  async list(params: InvoiceListParams) { return (await apiClient.get<BasePaginationResponse<InvoiceRequest>>('/admin/payment/invoices', { params })).data },
  async get(id: number) { return (await apiClient.get<InvoiceRequest>(`/admin/payment/invoices/${id}`)).data },
  async markIssued(ids: number[]) { return (await apiClient.post('/admin/payment/invoices/mark-issued', { ids })).data },
}
