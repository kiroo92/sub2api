import type { LuckyDrawConfig } from '@/api/luckyDraw'

export function luckyDrawRarity(amount: number): 'legendary' | 'epic' | 'rare' | 'common' | 'none' {
  return amount >= 20 ? 'legendary' : amount >= 5 ? 'epic' : amount >= 2 ? 'rare' : amount > 0 ? 'common' : 'none'
}

export function luckyDrawCost(config: LuckyDrawConfig) {
  const total = config.prizes.reduce((sum, p) => sum + Number(p.weight), 0)
  const regular = total > 0 ? config.prizes.reduce((sum, p) => sum + Number(p.amount) * Number(p.weight), 0) / total : 0
  const loss = total > 0 ? config.prizes.filter(p => p.amount === 0).reduce((sum, p) => sum + Number(p.weight), 0) / total : 1
  const denominator = config.guarantee_after > 0 ? 1 - loss ** config.guarantee_after : 1
  const guaranteed = denominator > 0 ? regular / denominator : 0
  const n = Math.max(0, Math.min(1000, Math.floor(Number(config.guarantee_after) || 0)))
  let states = new Array(Math.max(1, n)).fill(0); states[0] = 1
  let tenInitial = 0
  for (let draw = 0; draw < 10; draw++) {
    const next = new Array(states.length).fill(0)
    states.forEach((probability, misses) => {
      const forced = n > 0 && misses >= n - 1
      tenInitial += probability * (forced && loss < 1 ? regular / (1 - loss) : regular)
      next[0] += probability * (forced ? 1 : 1 - loss)
      if (!forced) next[n ? Math.min(misses + 1, n - 1) : 0] += probability * loss
    })
    states = next
  }
  return { regular, guaranteed, tenInitial, tenLongTerm: guaranteed * 10, tenRecharge: config.recharge_per_draw * 10, ratio: config.recharge_per_draw > 0 ? guaranteed / config.recharge_per_draw * 100 : 0 }
}

export function luckyDrawRequestID() {
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6]! & 15) | 64; bytes[8] = (bytes[8]! & 63) | 128
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}
