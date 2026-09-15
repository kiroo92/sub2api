<template>
  <div class="space-y-3">
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }} <button class="underline" @click="load">{{ t('common.refresh') }}</button></p>
    <p v-if="loading && !items.length" class="py-6 text-center">{{ t('common.loading') }}</p>
    <VueDraggable v-model="active" class="grid items-start gap-4 md:grid-cols-2 xl:grid-cols-3" handle=".package-drag" :disabled="saving || loading" :animation="150" @start="dragging = true" @end="saveOrder">
      <PackageHoldingCard v-for="(item, index) in active" :key="`package:${item.id}`" :item="item">
        <div class="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-2 dark:border-dark-700">
          <button type="button" class="package-drag inline-flex min-h-10 min-w-10 cursor-grab items-center justify-center rounded-md px-2 py-1 text-sm text-primary-600 focus-visible:ring-2 focus-visible:ring-primary-500" :title="t('packages.drag', { name: item.plan.name })" :aria-label="t('packages.drag', { name: item.plan.name })" :disabled="saving || loading"><Icon name="arrowsUpDown" size="sm" /><span class="ml-1">{{ t('packages.priority', { position: index + 1 }) }}</span></button>
          <div class="flex gap-2">
            <button type="button" class="btn btn-secondary inline-flex min-h-10 min-w-10 items-center justify-center px-3 py-1" :disabled="saving || loading || index === 0" :title="t('packages.moveUp', { name: item.plan.name })" :aria-label="t('packages.moveUp', { name: item.plan.name })" @click="move(index, -1)"><Icon name="arrowUp" size="sm" /></button>
            <button type="button" class="btn btn-secondary inline-flex min-h-10 min-w-10 items-center justify-center px-3 py-1" :disabled="saving || loading || index === active.length - 1" :title="t('packages.moveDown', { name: item.plan.name })" :aria-label="t('packages.moveDown', { name: item.plan.name })" @click="move(index, 1)"><Icon name="arrowDown" size="sm" /></button>
          </div>
        </div>
      </PackageHoldingCard>
    </VueDraggable>
    <p v-if="saving" role="status" class="text-sm text-gray-500">{{ t('packages.saving') }}</p>
    <details v-if="history.length" class="space-y-3"><summary class="cursor-pointer text-sm text-gray-500">{{ t('packages.showHistory') }} ({{ history.length }})</summary><div class="mt-3 grid items-start gap-4 md:grid-cols-2 xl:grid-cols-3"><PackageHoldingCard v-for="item in history" :key="`package:${item.id}`" :item="item" /></div></details>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { VueDraggable } from 'vue-draggable-plus'
import { useI18n } from 'vue-i18n'
import { packagesAPI } from '@/api/packages'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { UserPackage } from '@/types/packages'
import PackageHoldingCard from './PackageHoldingCard.vue'
import Icon from '@/components/icons/Icon.vue'
const { t } = useI18n()
const emit = defineEmits<{ loaded: [count: number]; error: [] }>()
const app = useAppStore()
const items = ref<UserPackage[]>([])
const active = ref<UserPackage[]>([])
const history = computed(() => items.value.filter(item => !active.value.some(entry => entry.id === item.id)))
const loading = ref(false)
const saving = ref(false)
const dragging = ref(false)
const error = ref('')
const savedOrder = ref<number[]>([])
let refreshPending = false
function refresh() {
  if (saving.value || loading.value || dragging.value) { refreshPending = true; return }
  return load()
}
async function load() {
  if (saving.value || loading.value || dragging.value) return
  refreshPending = false
  loading.value = true
  try {
    items.value = await packagesAPI.mine()
    active.value = items.value.filter(item => item.status === 'active' && Date.parse(item.expires_at) > Date.now())
    savedOrder.value = active.value.map(item => item.id)
    emit('loaded', items.value.length)
    error.value = ''
  } catch (err) { error.value = extractApiErrorMessage(err, t('packages.loadError')); emit('error') }
  finally {
    loading.value = false
    if (refreshPending) void load()
  }
}
async function saveOrder() {
  dragging.value = false
  if (saving.value || loading.value) return
  const nextOrder = active.value.map(item => item.id)
  if (nextOrder.join(',') === savedOrder.value.join(',')) {
    if (refreshPending) void load()
    return
  }
  const previousOrder = [...savedOrder.value]
  saving.value = true
  try {
    await packagesAPI.reorder(nextOrder)
    savedOrder.value = nextOrder
    error.value = ''
    app.showSuccess(t('packages.saved'))
  } catch (err) {
    const byID = new Map(items.value.map(item => [item.id, item]))
    active.value = previousOrder.map(id => byID.get(id)).filter((item): item is UserPackage => Boolean(item))
    error.value = extractApiErrorMessage(err, t('common.error'))
  }
  finally {
    saving.value = false
    const saveError = error.value
    await load()
    if (saveError) error.value = saveError
  }
}
async function move(index: number, offset: number) {
  if (saving.value || loading.value || dragging.value || index + offset < 0 || index + offset >= active.value.length) return
  const moved = [...active.value]
  const [item] = moved.splice(index, 1)
  moved.splice(index + offset, 0, item)
  active.value = moved
  await saveOrder()
}
onMounted(load)
useIntervalFn(load, 30000)
defineExpose({ refresh })
</script>
