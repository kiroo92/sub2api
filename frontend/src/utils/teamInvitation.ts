const pendingInvitationKey = 'pending_team_invitation'
const pendingActivationKey = 'pending_team_activation'

function validToken(token: string | null): token is string {
  return !!token && /^[a-f0-9]{64}$/.test(token)
}

export function rememberTeamInvitation(hash: string): boolean {
  const params = new URLSearchParams(hash.replace(/^#/, ''))
  const activation = params.get('activate')
  const token = params.get('invite')
  if (!validToken(activation) && !validToken(token)) return false
  // The latest link replaces any earlier invitation in this browser tab.
  sessionStorage.removeItem(pendingInvitationKey)
  sessionStorage.removeItem(pendingActivationKey)
  if (validToken(activation)) sessionStorage.setItem(pendingActivationKey, activation)
  else if (validToken(token)) sessionStorage.setItem(pendingInvitationKey, token)
  return true
}

export function hasPendingTeamInvitation(): boolean {
  return validToken(sessionStorage.getItem(pendingInvitationKey)) || validToken(sessionStorage.getItem(pendingActivationKey))
}

export function takeTeamInvitation(): string {
  const token = sessionStorage.getItem(pendingInvitationKey)
  sessionStorage.removeItem(pendingInvitationKey)
  return validToken(token) ? token : ''
}

export function getTeamActivation(): string {
  const token = sessionStorage.getItem(pendingActivationKey)
  return validToken(token) ? token : ''
}

export function clearTeamActivation() {
  sessionStorage.removeItem(pendingActivationKey)
}
