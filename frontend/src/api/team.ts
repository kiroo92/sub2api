import { apiClient } from './client'
import type { Group } from '@/types'

export interface TeamLimits { daily: number; weekly: number; monthly: number }
export interface Team {
  id: number; name: string; owner_id: number; status: 'active' | 'paused'
  group_ids: number[]; default_limits: TeamLimits; created_at: string
}
export interface TeamMember {
  username?: string
  user_id: number; email: string; role: 'owner' | 'member'; limits: TeamLimits
  usage: TeamLimits & { total: number }; resets: Record<keyof TeamLimits, string | null>; joined_at: string
}
export interface TeamInvitation {
  id: number; team_id: number; team_name: string; email: string; status: string; expires_at: string; created_at: string
}
export interface TeamKey {
  id: number; user_id: number; email: string; name: string; key: string
  status: 'active' | 'disabled'; quota_used: number; created_at: string; last_used_at: string | null
}
export interface TeamSnapshot {
  pending_billing?: number
  pending_requests?: number
  usage?: TeamLimits & { total: number }
  team: Team | null; role: 'owner' | 'member' | null; members: TeamMember[]
  invitations: TeamInvitation[]; pending_invitations: TeamInvitation[]
}

export function createTeamAPI(base = '/team') {
  return {
    recoverBilling: () => apiClient.post(`${base}/billing/recover`),
    get: async () => (await apiClient.get<TeamSnapshot>(base)).data,
    create: (name: string) => apiClient.post(base, { name }),
    update: (data: Partial<Pick<Team, 'name' | 'status' | 'group_ids' | 'default_limits'>>) => apiClient.put(base, data),
    dissolve: (confirm_name: string) => apiClient.delete(base, { data: { confirm_name } }),
    leave: () => apiClient.post(`${base}/leave`),
    invite: (email: string) => apiClient.post(`${base}/invitations`, { email }),
    resend: (id: number) => apiClient.post(`${base}/invitations/${id}/resend`),
    revoke: (id: number) => apiClient.delete(`${base}/invitations/${id}`),
    accept: (invitation_id: number) => apiClient.post(`${base}/invitations/accept`, { invitation_id }),
    acceptToken: (token: string) => apiClient.post(`${base}/invitations/accept`, { token }),
    limits: (userID: number, limits: TeamLimits) => apiClient.put(`${base}/members/${userID}/limits`, limits),
    remove: (userID: number) => apiClient.delete(`${base}/members/${userID}`),
    groups: async () => (await apiClient.get<Group[]>(`${base}/groups`)).data,
    keys: async () => (await apiClient.get<TeamKey[]>(`${base}/keys`)).data,
    createKey: (name: string) => apiClient.post(`${base}/keys`, { name }),
    updateKey: (id: number, data: { name?: string; status?: 'active' | 'disabled' }) => apiClient.put(`${base}/keys/${id}`, data),
    deleteKey: (id: number) => apiClient.delete(`${base}/keys/${id}`)
  }
}
export const teamAPI = createTeamAPI()

export interface TeamAdminItem { id: number; name: string; owner_id: number; owner_email: string; status: string; member_count: number; total_usage: number }
export const adminTeamsAPI = {
  list: async (params: { search: string; status: string; page: number; page_size: number }) => (await apiClient.get<{ items: TeamAdminItem[]; total: number }>('/admin/teams', { params })).data,
  config: async () => (await apiClient.get<{ frontend_url: string }>('/admin/teams/config')).data,
  saveConfig: (frontend_url: string) => apiClient.put('/admin/teams/config', { frontend_url })
}
