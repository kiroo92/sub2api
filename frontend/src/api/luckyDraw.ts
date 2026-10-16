import { apiClient } from './client'

export interface LuckyDrawPrize { name: string; amount: number; weight: number }
export interface LuckyDrawConfig {
  enabled: boolean
  version: number
  recharge_per_draw: number
  guarantee_after: number
  prizes: LuckyDrawPrize[]
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
  misses: number
  history: LuckyDrawRecord[]
}
export interface LuckyDrawRequest { request_id: string; config_version: number }
export const luckyDrawAPI = {
  async get(): Promise<LuckyDrawSnapshot> { return (await apiClient.get('/lucky-draw')).data },
  async draw(input: LuckyDrawRequest): Promise<LuckyDrawRecord> { return (await apiClient.post('/lucky-draw/draw', input)).data },
  async adminGet(): Promise<LuckyDrawSnapshot> { return (await apiClient.get('/admin/lucky-draw')).data },
  async configure(input: LuckyDrawConfig): Promise<LuckyDrawConfig> { return (await apiClient.put('/admin/lucky-draw/config', input)).data }
}
