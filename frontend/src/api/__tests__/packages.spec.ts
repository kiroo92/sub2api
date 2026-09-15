import { beforeEach, describe, expect, it, vi } from 'vitest'
import { create, keyRoutingUpdate } from '../keys'
import { packagesAPI } from '../packages'
import { buildCreateOrderPayload, decidePaymentLaunch, readPaymentRecoverySnapshot } from '@/components/payment/paymentFlow'
import { parseWechatResumeRoute } from '@/views/user/paymentWechatResume'
const { post, put } = vi.hoisted(() => ({ post: vi.fn(), put: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { post, put } }))

describe('independent package API identities', () => {
  beforeEach(() => { post.mockReset().mockResolvedValue({ data: {} }); put.mockReset().mockResolvedValue({ data: {} }) })
  it('keeps legacy key payload unchanged and never uses a synthetic group for package keys', async () => {
    await create('legacy', 42)
    expect(post).toHaveBeenLastCalledWith('/keys', { name: 'legacy', group_id: 42 })
    await create('package', 42, undefined, undefined, undefined, undefined, undefined, undefined, 'all_packages')
    expect(post).toHaveBeenLastCalledWith('/keys', { name: 'package', group_id: null, routing_mode: 'all_packages' })
    expect(keyRoutingUpdate(42)).toEqual({ group_id: 42 })
    expect(keyRoutingUpdate(42, 'all_packages')).toEqual({ group_id: 42, routing_mode: 'fixed_group' })
    expect(keyRoutingUpdate('all_packages')).toEqual({ group_id: null, routing_mode: 'all_packages' })
  })
  it('sends only package IDs to the package ordering endpoint', async () => {
    await packagesAPI.reorder([7, 2])
    expect(put).toHaveBeenCalledWith('/packages/order', { package_ids: [7, 2] })
  })
  it('retains package/group identities through checkout, WeChat and payment recovery', () => {
    expect(buildCreateOrderPayload({ amount: 95, paymentType: 'wxpay', orderType: 'package', planId: 99, packagePlanId: 7, groupBuyId: 8, isMobile: true, isWechatBrowser: true })).toEqual({
      amount: 95, payment_type: 'wxpay', order_type: 'package', package_plan_id: 7, group_buy_id: 8, is_mobile: true, payment_source: 'wechat_in_app_resume',
    })
    expect(parseWechatResumeRoute({ wechat_resume_token: 'signed', order_type: 'package', package_plan_id: '7', group_buy_id: '8' }, [], 0)).toMatchObject({ orderType: 'package', packagePlanId: 7, groupBuyId: 8, wechatResumeToken: 'signed' })
    const result = decidePaymentLaunch({ order_id: 10, amount: 95, pay_amount: 95, fee_rate: 0, qr_code: 'qr', expires_at: '2099-01-01T00:00:00Z' }, { visibleMethod: 'alipay', orderType: 'package', isMobile: false })
    expect(readPaymentRecoverySnapshot(JSON.stringify(result.recovery))?.orderType).toBe('package')
  })
})
