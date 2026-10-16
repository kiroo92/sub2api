<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <header><h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('luckyDraw.adminTitle') }}</h1><p class="mt-2 text-sm text-gray-500">{{ t('luckyDraw.adminDescription') }}</p></header>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <button v-if="!form" class="btn btn-secondary" :disabled="loading" @click="load">{{ loading ? t('luckyDraw.loading') : t('luckyDraw.refresh') }}</button>
      <form v-if="form" class="space-y-6" @submit.prevent="save">
        <fieldset class="card space-y-5 p-6" :disabled="saving || loading">
          <label class="flex items-center justify-between gap-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><span class="font-medium text-gray-900 dark:text-white">{{ t('luckyDraw.enabled') }}</span><input v-model="form.enabled" type="checkbox" class="h-5 w-5 rounded" /></label>
          <div class="grid gap-5 sm:grid-cols-2">
            <label class="input-label">{{ t('luckyDraw.threshold') }} ($)<input v-model.number="form.recharge_per_draw" class="input mt-2" type="number" min="0.01" max="1000000" step="0.01" required /></label>
            <label class="input-label">{{ t('luckyDraw.guaranteeAfter') }}<input v-model.number="form.guarantee_after" class="input mt-2" type="number" min="0" max="1000" step="1" required /></label>
          </div>
          <p class="text-xs leading-6 text-gray-500">{{ t('luckyDraw.configHint') }}</p>
        </fieldset>
        <fieldset class="card p-6" :disabled="saving || loading">
          <div class="flex flex-wrap items-center justify-between gap-3"><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('luckyDraw.prizes') }}</h2><button type="button" class="btn btn-secondary" :disabled="form.prizes.length >= 12" @click="form.prizes.push({ name: '', amount: 0, weight: 1 })">{{ t('luckyDraw.addPrize') }}</button></div>
          <div class="mt-5 space-y-4">
            <div v-for="(prize, index) in form.prizes" :key="index" class="grid gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-800 sm:grid-cols-7">
              <label class="input-label sm:col-span-3">{{ t('luckyDraw.prizeName') }}<input v-model="prize.name" class="input mt-2" maxlength="80" required /></label>
              <label class="input-label sm:col-span-2">{{ t('luckyDraw.amount') }} ($)<input v-model.number="prize.amount" class="input mt-2" type="number" min="0" max="10000" step="0.01" required /></label>
              <label class="input-label">{{ t('luckyDraw.weight') }}<input v-model.number="prize.weight" class="input mt-2" type="number" min="1" max="10000" step="1" required /></label>
              <button type="button" class="btn btn-secondary self-end" :disabled="form.prizes.length <= 2" :aria-label="t('luckyDraw.removePrize')" @click="form.prizes.splice(index, 1)">×</button>
            </div>
          </div>
          <p class="mt-4 text-sm" :class="weightTotal === 10000 ? 'text-emerald-600' : 'text-red-600'">{{ t('luckyDraw.weightTotal', { total: weightTotal }) }}</p>
          <p class="mt-2 text-xs leading-6 text-gray-500">{{ t('luckyDraw.weightHint') }}</p>
        </fieldset>
        <div class="flex flex-wrap items-center gap-4"><button class="btn btn-primary" :disabled="saving || loading || weightTotal !== 10000" type="submit">{{ saving ? t('luckyDraw.saving') : t('luckyDraw.save') }}</button><button type="button" class="btn btn-secondary" :disabled="saving || loading" @click="load">{{ t('luckyDraw.refresh') }}</button><p v-if="saved" role="status" class="text-sm text-emerald-600">{{ t('luckyDraw.saved') }}</p><router-link to="/lucky-draw" class="text-sm text-primary-600 hover:underline">{{ t('luckyDraw.preview') }} →</router-link></div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { luckyDrawAPI, type LuckyDrawConfig } from '@/api/luckyDraw'
const { t } = useI18n()
const form = ref<LuckyDrawConfig | null>(null)
const loading = ref(false)
const saving = ref(false)
const saved = ref(false)
const error = ref('')
const weightTotal = computed(() => form.value?.prizes.reduce((sum, prize) => sum + Number(prize.weight), 0) ?? 0)
async function load() {
  loading.value = true; error.value = ''
  try { form.value = (await luckyDrawAPI.adminGet()).config }
  catch { error.value = t('luckyDraw.loadFailed') }
  finally { loading.value = false }
}
async function save() {
  if (saving.value || !form.value) return
  saving.value = true; saved.value = false; error.value = ''
  try { form.value = await luckyDrawAPI.configure(form.value); saved.value = true }
  catch (err: unknown) {
    const reason = (err as { reason?: string }).reason
    error.value = reason === 'LUCKY_DRAW_THRESHOLD_LOCKED' ? t('luckyDraw.thresholdLocked') : reason === 'LUCKY_DRAW_CHANGED' ? t('luckyDraw.changed') : reason === 'INVALID_LUCKY_DRAW_CONFIG' ? t('luckyDraw.invalidConfig') : t('luckyDraw.saveFailed')
  } finally { saving.value = false }
}
onMounted(load)
</script>
