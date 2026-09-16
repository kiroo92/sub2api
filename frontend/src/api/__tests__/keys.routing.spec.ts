import { describe, expect, it } from 'vitest'
import { keyRoutingUpdate } from '../keys'

describe('key routing payload', () => {
  it('keeps modes mutually exclusive on switch', () => {
    expect(keyRoutingUpdate('all_subscriptions', 'fixed_group')).toEqual({ routing_mode: 'all_subscriptions', group_id: null })
    expect(keyRoutingUpdate(7, 'all_subscriptions')).toEqual({ routing_mode: 'fixed_group', group_id: 7 })
    expect(keyRoutingUpdate(7, 'fixed_group')).toEqual({ group_id: 7 })
  })
})
