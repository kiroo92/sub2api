import { apiClient } from './client'
import type { PackageGroupBuy, PackagePlan, UserPackage } from '@/types/packages'

export const packagesAPI = {
  async plans() { return (await apiClient.get<PackagePlan[]>('/packages/plans')).data },
  async mine() { return (await apiClient.get<UserPackage[]>('/packages/mine')).data },
  async reorder(packageIDs: number[]) {
    await apiClient.put('/packages/order', { package_ids: packageIDs })
  },
  async groups() { return (await apiClient.get<PackageGroupBuy[]>('/packages/groups')).data },
  async group(id: number) { return (await apiClient.get<PackageGroupBuy>(`/packages/groups/${id}`)).data },
  async startGroup(planID: number) {
    return (await apiClient.post<PackageGroupBuy>('/packages/groups', { plan_id: planID })).data
  },
  async adminPlans() { return (await apiClient.get<PackagePlan[]>('/admin/packages/plans')).data },
  async savePlan(plan: PackagePlan) {
    return plan.id
      ? (await apiClient.put<PackagePlan>(`/admin/packages/plans/${plan.id}`, plan)).data
      : (await apiClient.post<PackagePlan>('/admin/packages/plans', plan)).data
  },
  async unpublish(id: number) { await apiClient.delete(`/admin/packages/plans/${id}`) }
}
