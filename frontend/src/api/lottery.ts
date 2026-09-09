import { apiClient } from './client'

export interface LotteryConfig {
  turnstile_site_key?: string
  turnstile_secret_configured?: boolean
  turnstile_secret_key?: string
  enabled: boolean
  prize_amount: number
  winner_count: number
  participant_target: number
  min_recharge: number
}
export interface LotteryRound {
  id: number
  prize_amount: number
  winner_count: number
  participant_target: number
  min_recharge: number
  participant_count: number
  winners_drawn: number
  status: 'open' | 'drawn'
  created_at: string
  drawn_at: string | null
}
export interface LotteryWin {
  round_id: number
  user_label: string
  prize_amount: number
  awarded_at: string
}
export interface LotterySnapshot {
  config: LotteryConfig
  current: LotteryRound | null
  joined: boolean
  eligible: boolean
  total_recharged: number
  recent_winners: LotteryWin[]
  my_wins: LotteryWin[]
  recent_rounds: LotteryRound[]
}
export interface LotteryCaptchaProof { turnstile_token: string }
export const lotteryAPI = {
  async get(): Promise<LotterySnapshot> { return (await apiClient.get('/lottery')).data },
  async join(roundID: number, proof: LotteryCaptchaProof): Promise<{ round_id: number; already_joined: boolean; drawn: boolean }> {
    return (await apiClient.post('/lottery/join', { round_id: roundID, ...proof })).data
  },
  async adminGet(): Promise<LotterySnapshot> { return (await apiClient.get('/admin/lottery')).data },
  async configure(config: LotteryConfig): Promise<LotteryConfig> {
    return (await apiClient.put('/admin/lottery/config', config)).data
  }
}
