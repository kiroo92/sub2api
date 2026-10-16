import { apiClient } from './client'

export interface LuckyDrawPrize { name: string; amount: number; weight: number }
export interface LuckyDrawConfig {
  enabled: boolean
  version: number
  recharge_per_draw: number
  guarantee_after: number
  prizes: LuckyDrawPrize[]
  recharge_starts_at: string
}
export interface LuckyDrawRecord {
  id: number
  request_id: string
  config_version: number
  prize_index: number
  prize_name: string
  prize_amount: number
  guaranteed: boolean
  created_at: string
}
export interface LuckyDrawSnapshot {
  config: LuckyDrawConfig
  total_recharged: number
  chances: number
  draws_used: number
  manual_chances: number
  misses: number
  history: LuckyDrawRecord[]
}
export interface LuckyDrawRequest { request_id: string; config_version: number; count?: 1 | 5 | 10 }
export interface LuckyDrawBatch { request_id: string; records: LuckyDrawRecord[]; total_awarded: number }
export interface LuckyDrawGrant { id: number; user_id: number; admin_id: number; chances: number; note: string; created_at: string }
export interface LuckyDrawTotals {
  recharge_amount: number
  chances_earned: number
  manual_chances: number
  draws_used: number
  chances: number
  wins: number
  prize_amount: number
}
export interface LuckyDrawAdminStats {
  recharge_starts_at: string
  summary: LuckyDrawTotals
  users: (LuckyDrawTotals & { user_id: number; email: string })[]
  prizes: { name: string; amount: number; draws: number; total_awarded: number }[]
  total: number
  page: number
  page_size: number
  grants: LuckyDrawGrant[]
}
export const luckyDrawAPI = {
  async get(): Promise<LuckyDrawSnapshot> { return (await apiClient.get('/lucky-draw')).data },
  async draw(input: LuckyDrawRequest): Promise<LuckyDrawRecord> { return (await apiClient.post('/lucky-draw/draw', input)).data },
  async drawBatch(input: LuckyDrawRequest): Promise<LuckyDrawBatch> { return (await apiClient.post('/lucky-draw/draw', input)).data },
  async grant(input: { user_id: number; count: number; note: string; request_id: string }): Promise<LuckyDrawGrant> { return (await apiClient.post('/admin/lucky-draw/grants', input)).data },
  async adminGet(): Promise<LuckyDrawSnapshot> { return (await apiClient.get('/admin/lucky-draw')).data },
  async stats(search = '', page = 1): Promise<LuckyDrawAdminStats> { return (await apiClient.get('/admin/lucky-draw/stats', { params: { search, page } })).data },
  async configure(input: LuckyDrawConfig): Promise<LuckyDrawConfig> { return (await apiClient.put('/admin/lucky-draw/config', input)).data }
}
