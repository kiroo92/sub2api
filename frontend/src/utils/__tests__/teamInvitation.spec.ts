import { beforeEach, expect, it } from 'vitest'
import { rememberTeamInvitation, takeTeamInvitation, hasPendingTeamInvitation, getTeamActivation, clearTeamActivation } from '../teamInvitation'

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

it('preserves activation links through login and refresh until activation succeeds', () => {
  const token = 'b'.repeat(64)
  expect(rememberTeamInvitation(`#activate=${token}`)).toBe(true)
  expect(hasPendingTeamInvitation()).toBe(true)
  expect(takeTeamInvitation()).toBe('')
  expect(getTeamActivation()).toBe(token)
  expect(getTeamActivation()).toBe(token)
  clearTeamActivation()
  expect(hasPendingTeamInvitation()).toBe(false)
})
it('uses the latest valid link and rejects malformed activation links', () => {
  const token = 'c'.repeat(64)
  expect(rememberTeamInvitation(`#activate=${token}`)).toBe(true)
  expect(rememberTeamInvitation('#activate=invalid')).toBe(false)
  expect(getTeamActivation()).toBe(token)
  expect(rememberTeamInvitation(`#invite=${token}`)).toBe(true)
  expect(getTeamActivation()).toBe('')
  expect(takeTeamInvitation()).toBe(token)
})
