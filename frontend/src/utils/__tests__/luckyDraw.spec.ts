import { describe, expect, it } from 'vitest'
import { luckyDrawCost, luckyDrawRarity } from '../luckyDraw'
import type { LuckyDrawConfig } from '@/api/luckyDraw'

const config: LuckyDrawConfig = {
  enabled: true, version: 1, recharge_starts_at: '2026-10-15T16:00:00Z', recharge_per_draw: 5, guarantee_after: 10,
  prizes: [{ name: '20', amount: 20, weight: 5 }, { name: '5', amount: 5, weight: 50 }, { name: '2', amount: 2, weight: 200 }, { name: '1', amount: 1, weight: 700 }, { name: '0.5', amount: 0.5, weight: 2000 }, { name: '0', amount: 0, weight: 7045 }]
}
describe('Lucky draw estimates', () => {
  it('includes the forced positive outcome in the long-run expectation', () => {
    const cost = luckyDrawCost(config)
    expect(cost.regular).toBeCloseTo(0.245, 10)
    expect(cost.guaranteed).toBeCloseTo(0.245 / (1 - 0.7045 ** 10), 10)
    expect(cost.tenInitial).toBeCloseTo(2.4749700018, 8)
    expect(cost.tenLongTerm).toBeCloseTo(2.5260775745, 8)
    expect(cost.tenRecharge).toBe(50)
    expect(cost.ratio).toBeCloseTo(5.052155, 5)
    expect(luckyDrawCost({ ...config, guarantee_after: 0 }).guaranteed).toBe(0.245)
    expect(luckyDrawCost({ ...config, guarantee_after: 1 }).guaranteed).toBeCloseTo(0.245 / 0.2955, 10)
  })
  it.each([[0, 'none'], [0.5, 'common'], [1, 'common'], [2, 'rare'], [5, 'epic'], [20, 'legendary']])('assigns amount %s to %s', (amount, rarity) => {
    expect(luckyDrawRarity(Number(amount))).toBe(rarity)
  })
})
