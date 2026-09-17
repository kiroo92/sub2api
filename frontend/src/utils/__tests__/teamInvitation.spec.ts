import { beforeEach, expect, it } from 'vitest'
import { rememberTeamInvitation, takeTeamInvitation, hasPendingTeamInvitation } from '../teamInvitation'

beforeEach(() => sessionStorage.clear())
it('preserves a valid invitation through login/registration and consumes it once', () => {
  const token = 'a'.repeat(64)
  expect(rememberTeamInvitation(`#invite=${token}`)).toBe(true)
  expect(hasPendingTeamInvitation()).toBe(true)
  expect(rememberTeamInvitation('')).toBe(false)
  expect(takeTeamInvitation()).toBe(token)
  expect(hasPendingTeamInvitation()).toBe(false)
  expect(takeTeamInvitation()).toBe('')
})
it('rejects malformed tokens and keeps unrelated hash state out of storage', () => {
  for (const hash of ['#other=1', '#invite=javascript:alert(1)', '#invite=short', `#invite=${'a'.repeat(65)}`]) expect(rememberTeamInvitation(hash)).toBe(false)
  expect(hasPendingTeamInvitation()).toBe(false)
})
