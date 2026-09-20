import type { SubscriptionBulkActionRequest } from '@/api/admin/subscriptions'
import type { AssignSubscriptionRequest, BulkAssignSubscriptionRequest } from '@/types'

export interface BulkSubscriptionOperation {
  request: SubscriptionBulkActionRequest
  key: string
  storageKey: string | null
  outcomeUncertain: boolean
}

const pendingKeys = new Map<string, string>()

function currentAdminId(): number | null {
  try {
    const user = JSON.parse(globalThis.localStorage?.getItem('auth_user') ?? 'null') as { id?: unknown } | null
    const id = user?.id
    return typeof id === 'number' && Number.isSafeInteger(id) && id > 0 ? id : null
  } catch {
    return null
  }
}

function readStoredKey(storageKey: string): string | null {
  try {
    return globalThis.sessionStorage?.getItem(storageKey) ?? null
  } catch {
    return null
  }
}

function storeKey(storageKey: string, key: string | null) {
  try {
    if (key) globalThis.sessionStorage?.setItem(storageKey, key)
    else globalThis.sessionStorage?.removeItem(storageKey)
  } catch {
    // Keep same-session retries safe in memory when browser storage is unavailable.
  }
}

export function prepareBulkSubscriptionOperation(input: SubscriptionBulkActionRequest): BulkSubscriptionOperation {
  // Send the same canonical payload used for storage, including ID order, so a
  // retry from a differently sorted table still matches the backend fingerprint.
  const request: SubscriptionBulkActionRequest = {
    subscription_ids: [...new Set(input.subscription_ids)].sort((a, b) => a - b),
    action: input.action
  }
  if (input.action === 'extend') request.days = input.days
  if (input.action === 'reset_quota') {
    request.daily = !!input.daily
    request.weekly = !!input.weekly
    request.monthly = !!input.monthly
  }
  const adminId = currentAdminId()
  const storageKey = adminId ? `sub2api:admin:subscription-bulk:${adminId}:${JSON.stringify(request)}` : null
  let key = storageKey ? pendingKeys.get(storageKey) ?? readStoredKey(storageKey) : null
  const outcomeUncertain = !!key
  if (!key) {
    const requestId = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
    key = `subscription-bulk-${adminId ?? 'unknown'}-${requestId}`
  }
  if (storageKey) {
    pendingKeys.set(storageKey, key)
    storeKey(storageKey, key)
  }
  return { request, key, storageKey, outcomeUncertain }
}

export function completeBulkSubscriptionOperation(operation: Pick<BulkSubscriptionOperation, 'storageKey'>) {
  if (!operation.storageKey) return
  pendingKeys.delete(operation.storageKey)
  storeKey(operation.storageKey, null)
}

// Preserve an uncertain grant across dialog/page reopen; success allows a new grant.
export function prepareSubscriptionAssignment<T extends AssignSubscriptionRequest | BulkAssignSubscriptionRequest>(input: T) {
  const request = { ...input, ...('user_ids' in input ? { user_ids: [...new Set(input.user_ids)].sort((a, b) => a - b) } : {}) }
  const adminId = currentAdminId()
  const storageKey = adminId ? `sub2api:admin:subscription-assign:${adminId}:${JSON.stringify(request)}` : null
  let key = storageKey ? pendingKeys.get(storageKey) ?? readStoredKey(storageKey) : null
  if (!key) key = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
  if (storageKey) { pendingKeys.set(storageKey, key); storeKey(storageKey, key) }
  return { request, key, storageKey }
}
