const pendingInvitationKey = 'pending_team_invitation'

export function rememberTeamInvitation(hash: string): boolean {
  const token = new URLSearchParams(hash.replace(/^#/, '')).get('invite')
  if (!token || !/^[a-f0-9]{64}$/.test(token)) return false
  sessionStorage.setItem(pendingInvitationKey, token)
  return true
}

export function hasPendingTeamInvitation(): boolean {
  return /^[a-f0-9]{64}$/.test(sessionStorage.getItem(pendingInvitationKey) || '')
}

export function takeTeamInvitation(): string {
  const token = hasPendingTeamInvitation() ? sessionStorage.getItem(pendingInvitationKey)! : ''
  sessionStorage.removeItem(pendingInvitationKey)
  return token
}
