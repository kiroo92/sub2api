import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { lotteryAPI } from '@/api/lottery'
import { useAuthStore } from './auth'

const CACHE_TTL_MS = 30_000

export const useLotteryStore = defineStore('lottery', () => {
  const authStore = useAuthStore()
  const scope = computed(() =>
    authStore.isAuthenticated && !authStore.isSimpleMode ? authStore.user?.id ?? null : null,
  )
  const configuredEnabled = ref(false)
  const enabled = computed(() => scope.value !== null && configuredEnabled.value)
  let fetchedAt: number | null = null
  let generation = 0
  let pending: Promise<void> | null = null

  function reset() {
    generation++
    pending = null
    fetchedAt = null
    configuredEnabled.value = false
  }
  watch(scope, reset, { flush: 'sync' })

  // Admin saves update navigation immediately and supersede older requests.
  function setEnabled(value: boolean) {
    reset()
    if (scope.value === null) return
    configuredEnabled.value = value === true
    fetchedAt = Date.now()
  }

  async function refresh() {
    if (scope.value === null) return
    if (pending) return pending
    if (fetchedAt !== null && Date.now() - fetchedAt < CACHE_TTL_MS) return
    const currentGeneration = ++generation
    pending = lotteryAPI.get()
      .then(snapshot => {
        if (generation === currentGeneration) {
          configuredEnabled.value = snapshot.config.enabled === true
          fetchedAt = Date.now()
        }
      })
      .catch(() => {
        // Unknown availability must not advertise a disabled activity.
        if (generation === currentGeneration) {
          configuredEnabled.value = false
          fetchedAt = Date.now()
        }
      })
      .finally(() => {
        if (generation === currentGeneration) pending = null
      })
    return pending
  }

  return { enabled, refresh, setEnabled }
})
