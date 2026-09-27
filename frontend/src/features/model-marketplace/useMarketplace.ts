import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { getModelPlaza, type ModelPlazaResponse } from '@/api/modelPlaza'
import {
  getMatrix,
  getSnapshot,
  type MonitorFilter,
  type MonitorMatrixResponse,
  type MonitorRange,
} from '@/api/channelMonitorV2'
import { useAppStore } from '@/stores/app'
import { isChannelMonitorV2Mode } from '@/utils/featureFlags'

export function useMarketplace() {
  const appStore = useAppStore()
  const catalog = ref<ModelPlazaResponse | null>(null)
  const matrix = ref<MonitorMatrixResponse | null>(null)
  const range = ref<MonitorRange>('24h')
  const catalogLoading = ref(false)
  const catalogError = ref(false)
  const monitorLoading = ref(false)
  const monitorError = ref(false)
  const monitorEnabled = computed(() => isChannelMonitorV2Mode())
  const intervalSeconds = ref(60)
  let catalogController: AbortController | null = null
  let monitorController: AbortController | null = null
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false

  async function loadCatalog() {
    catalogController?.abort()
    const controller = new AbortController()
    catalogController = controller
    catalogLoading.value = true
    catalogError.value = false
    try {
      const response = await getModelPlaza({ signal: controller.signal })
      if (!controller.signal.aborted) catalog.value = response
    } catch {
      if (!controller.signal.aborted) catalogError.value = true
    } finally {
      if (!controller.signal.aborted) catalogLoading.value = false
    }
  }

  function scheduleRefresh() {
    clearTimeout(timer)
    if (disposed || !monitorEnabled.value) return
    timer = setTimeout(() => {
      if (document.hidden) scheduleRefresh()
      else void loadMonitor()
    }, intervalSeconds.value * 1000)
  }

  async function loadMonitor(reset = false) {
    monitorController?.abort()
    clearTimeout(timer)
    if (reset) matrix.value = null
    if (disposed || !monitorEnabled.value) {
      matrix.value = null
      monitorError.value = false
      monitorLoading.value = false
      return
    }
    const controller = new AbortController()
    monitorController = controller
    const filter: MonitorFilter = { range: range.value, platforms: [], groupIds: [], models: [] }
    monitorLoading.value = true
    monitorError.value = false
    // Use user endpoints even for an administrator's personal menu. Each API
    // applies its own visibility rules; catalog groups remain the source of rows.
    const results = await Promise.allSettled([
      getMatrix(filter, 'platform_group', false, controller.signal),
      getSnapshot(filter, false, controller.signal),
    ])
    if (controller.signal.aborted || disposed) return
    const [matrixResult, snapshotResult] = results
    if (matrixResult.status === 'fulfilled') matrix.value = matrixResult.value
    else monitorError.value = true
    if (snapshotResult.status === 'fulfilled') {
      intervalSeconds.value = snapshotResult.value.coverage.bootstrap?.active
        ? 10
        : Math.max(10, snapshotResult.value.config.refresh_interval_seconds || 60)
    }
    monitorLoading.value = false
    scheduleRefresh()
  }

  function refresh() {
    void loadCatalog()
    void loadMonitor()
  }
  function onVisibilityChange() {
    if (!document.hidden && !monitorLoading.value) void loadMonitor()
  }
  watch([range, monitorEnabled], () => void loadMonitor(true))
  onMounted(async () => {
    document.addEventListener('visibilitychange', onVisibilityChange)
    if (!appStore.publicSettingsLoaded) await appStore.fetchPublicSettings().catch(() => null)
    if (disposed) return
    refresh()
  })
  onBeforeUnmount(() => {
    disposed = true
    catalogController?.abort()
    monitorController?.abort()
    clearTimeout(timer)
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })
  return {
    catalog,
    matrix,
    range,
    catalogLoading,
    catalogError,
    monitorLoading,
    monitorError,
    monitorEnabled,
    loadCatalog,
    loadMonitor,
    refresh,
  }
}
