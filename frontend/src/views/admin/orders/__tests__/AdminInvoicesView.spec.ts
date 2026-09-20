import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import AdminInvoicesView from '../AdminInvoicesView.vue'
import type { InvoiceRequest } from '@/types/invoice'

const mocks = vi.hoisted(() => ({ config: vi.fn(), saveConfig: vi.fn(), list: vi.fn(), mark: vi.fn(), workbook: vi.fn(), save: vi.fn() }))
vi.mock('@/api/invoices', () => ({ adminInvoiceAPI: { config: mocks.config, saveConfig: mocks.saveConfig, list: mocks.list, markIssued: mocks.mark } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('../invoiceExport', () => ({ createInvoiceWorkbook: mocks.workbook }))
vi.mock('file-saver', () => ({ saveAs: mocks.save }))
const invoice = (id: number): InvoiceRequest => ({
  id, user_id: 1, status: 'pending', tax_id: '00123', title: 'Example', email: 'billing@example.com', remarks: '', created_at: '', expires_at: '', submitted_at: '', issued_at: null, issued_by: null,
  quote: { orders: [], currency: 'CNY', base_amount: 100, service_fee: 40, total_amount: 140, net_amount: 140, tax_amount: 0, tax_rate: 0, item_name: 'Service', tier: { upper_amount: null, type: 'fixed', value: 40 }, fingerprint: '' },
})
let wrapper: VueWrapper
beforeEach(() => {
  vi.clearAllMocks()
  mocks.config.mockResolvedValue({ enabled: false, item_name: 'Service', tax_rate: 3, tiers: [{ upper_amount: null, type: 'percentage', value: 3 }] })
  mocks.list.mockResolvedValue({ items: [invoice(1)], total: 1 })
  mocks.mark.mockResolvedValue({ success: true })
  mocks.workbook.mockResolvedValue({ workbook: {}, XLSX: { write: () => new Uint8Array([1]) } })
})
afterEach(() => wrapper?.unmount())
async function open() {
  wrapper = mount(AdminInvoicesView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, DataTable: true, Pagination: true,
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' }, InvoicePreview: true,
  } } })
  await flushPromises(); return wrapper
}
describe('invoice administration', () => {
  it('exports every page matching the filters without marking anything issued', async () => {
    const w = await open()
    mocks.list.mockImplementation(async (params: { page: number }) => ({ items: params.page === 1 ? Array.from({ length: 100 }, (_, i) => invoice(i + 1)) : [invoice(101)], total: 101 }))
    await w.findAll('button').find(b => b.text() === 'invoices.exportFiltered')!.trigger('click'); await flushPromises()
    expect(mocks.list).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, page_size: 100, status: 'pending' }))
    expect(mocks.workbook.mock.calls[0][0]).toHaveLength(101)
    expect(mocks.save).toHaveBeenCalledOnce()
    expect(mocks.mark).not.toHaveBeenCalled()
  })
  it('requires confirmation before marking the selected paid applications', async () => {
    const w = await open()
    w.getComponent({ name: 'DataTable' }).vm.$emit('update:selectedKeys', [1])
    await flushPromises()
    await w.findAll('button').find(b => b.text() === 'invoices.markIssued')!.trigger('click')
    expect(mocks.mark).not.toHaveBeenCalled()
    await w.findAll('button').find(b => b.text() === 'common.confirm')!.trigger('click'); await flushPromises()
    expect(mocks.mark).toHaveBeenCalledWith([1])
  })
})
