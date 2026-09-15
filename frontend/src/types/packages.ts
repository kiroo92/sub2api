export interface PackageTier {
  members: number
  quota_usd: number
}

export const PACKAGE_THEME_COLORS = {
  violet: { accent: '#7c3aed', soft: '#ede9fe', border: '#c4b5fd' },
  emerald: { accent: '#047857', soft: '#d1fae5', border: '#6ee7b7' },
  blue: { accent: '#1d4ed8', soft: '#dbeafe', border: '#93c5fd' },
  orange: { accent: '#c2410c', soft: '#ffedd5', border: '#fdba74' },
  rose: { accent: '#be123c', soft: '#ffe4e6', border: '#fda4af' },
  cyan: { accent: '#0e7490', soft: '#cffafe', border: '#67e8f9' },
  amber: { accent: '#b45309', soft: '#fef3c7', border: '#fcd34d' },
  indigo: { accent: '#4338ca', soft: '#e0e7ff', border: '#a5b4fc' },
} as const

export type PackageThemeColor = keyof typeof PACKAGE_THEME_COLORS
export const PACKAGE_THEME_COLOR_IDS = Object.keys(PACKAGE_THEME_COLORS) as PackageThemeColor[]

export function resolvePackageThemeColor(themeColor: string | null | undefined, planID: number): PackageThemeColor {
  if (themeColor && themeColor in PACKAGE_THEME_COLORS) return themeColor as PackageThemeColor
  const index = Math.abs(Math.trunc(planID)) % PACKAGE_THEME_COLOR_IDS.length
  return PACKAGE_THEME_COLOR_IDS[index]
}

export interface PackagePlan {
  id: number
  group_id: number
  group_name: string
  group_platform: string
  name: string
  description: string
  price: number
  currency: string
  validity_days: 7 | 30
  base_quota_usd: number
  group_buy_enabled: boolean
  group_buy_hours: number
  tiers: PackageTier[]
  theme_color?: PackageThemeColor
  for_sale: boolean
  sort_order: number
}

export interface PackagePeriod {
  id: number
  package_id: number
  period_index: number
  starts_at: string
  ends_at: string
  quota_usd: number
  used_usd: number
}

export interface UserPackage {
  id: number
  user_id: number
  group_id: number
  order_id: number
  plan: PackagePlan
  status: string
  starts_at: string
  expires_at: string
  sort_order: number
  group_buy_id: number | null
  current_period: PackagePeriod | null
  periods: PackagePeriod[]
}

export interface PackageGroupBuy {
  id: number
  plan: PackagePlan
  status: 'draft' | 'open' | 'settled'
  paid_count: number
  target_members: number
  starts_at: string | null
  ends_at: string | null
  settled_at: string | null
  final_members: number | null
  final_quota_usd: number | null
  joined: boolean
  order_id: number | null
}
