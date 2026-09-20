import { describe, expect, it } from 'vitest'
import { createInvoiceWorkbook } from '../invoiceExport'
import type { InvoiceRequest } from '@/types/invoice'

describe('invoice workbook', () => {
  it('keeps identifiers and buyer text as text cells and includes fee/tax and source details', async () => {
    const invoice: InvoiceRequest = {
      id: 7, user_id: 3, status: 'pending', tax_id: '00123456789012345678', title: '=1+1', email: 'billing@example.com', remarks: '+SUM(A1:A3)',
      created_at: '2026-09-20T00:00:00Z', expires_at: '2026-09-21T00:00:00Z', submitted_at: '2026-09-20T00:01:00Z', issued_at: null, issued_by: null,
      quote: { orders: [{ id: 12, order_no: '00001234567890123456789', order_type: 'balance', name: '', amount: 311.5 }], currency: 'CNY', base_amount: 311.5, service_fee: 38, total_amount: 349.5, net_amount: 339.32, tax_amount: 10.18, tax_rate: 3, item_name: 'Technical service', tier: { upper_amount: null, type: 'fixed', value: 38 }, fingerprint: '' },
    }
    const { workbook, XLSX } = await createInvoiceWorkbook([invoice], key => key.split('.').at(-1)!)
    const encoded = XLSX.write(workbook, { type: 'array', bookType: 'xlsx' })
    const read = XLSX.read(encoded, { type: 'array' })
    expect(read.Sheets.invoiceSheet.C2).toMatchObject({ t: 's', v: '=1+1' })
    expect(read.Sheets.invoiceSheet.C2.f).toBeUndefined()
    expect(read.Sheets.invoiceSheet.D2).toMatchObject({ t: 's', v: invoice.tax_id })
    expect(read.Sheets.invoiceSheet.G2.v).toBe(38)
    expect(read.Sheets.invoiceSheet.H2.v).toBe(349.5)
    expect(read.Sheets.ordersSheet.C2).toMatchObject({ t: 's', v: '00001234567890123456789' })
    expect(invoice.status).toBe('pending')
  })
})
