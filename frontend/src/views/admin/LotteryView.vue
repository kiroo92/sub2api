<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div v-if="error" role="alert" class="rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }} <button class="ml-4 underline" :disabled="loading" @click="load">{{ t('lottery.refresh') }}</button></div>
      <div v-if="loading && !snapshot" role="status" class="card p-12 text-center text-gray-500">{{ t('lottery.loading') }}</div>
      <template v-if="snapshot">
        <div class="grid items-start gap-6 lg:grid-cols-5">
          <form class="card space-y-6 p-6 lg:col-span-3" @submit.prevent="save">
            <div><h1 class="text-xl font-bold text-gray-900 dark:text-gray-100">{{ t('lottery.adminTitle') }}</h1><p class="mt-2 text-sm leading-relaxed text-gray-500">{{ t('lottery.adminDescription') }}</p></div>
            <label class="flex cursor-pointer items-center justify-between gap-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><span><span class="block text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('lottery.enabled') }}</span><span class="mt-1 block text-xs text-gray-500">{{ t('lottery.pauseHint') }}</span></span><input v-model="form.enabled" type="checkbox" class="h-5 w-5 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :disabled="saving" /></label>
            <div class="grid gap-5 sm:grid-cols-2">
              <div><label for="prize-amount" class="input-label">{{ t('lottery.prizeAmount') }} ($)</label><input id="prize-amount" v-model.number="form.prize_amount" type="number" min="0.01" max="10000" step="0.01" required class="input" :disabled="saving" /></div>
              <div><label for="winner-count" class="input-label">{{ t('lottery.prizes') }}</label><input id="winner-count" v-model.number="form.winner_count" type="number" min="1" :max="Math.min(100, form.participant_target)" step="1" required class="input" :disabled="saving" /></div>
              <div><label for="participant-target" class="input-label">{{ t('lottery.participantTarget') }}</label><input id="participant-target" v-model.number="form.participant_target" type="number" :min="Math.max(2, form.winner_count)" max="10000" step="1" required class="input" :disabled="saving" /></div>
              <div><label for="min-recharge" class="input-label">{{ t('lottery.minRecharge') }} ($)</label><input id="min-recharge" v-model.number="form.min_recharge" type="number" min="0" max="1000000" step="0.01" required class="input" :disabled="saving" /></div>
            </div>
            <p class="text-sm text-gray-500">{{ t('lottery.budget', { amount: budget }) }}</p>
            <div class="rounded-xl bg-primary-50 p-4 text-sm leading-relaxed text-primary-800 dark:bg-primary-950/30 dark:text-primary-300">{{ t('lottery.nextRoundHint') }}</div>
            <div class="flex flex-wrap items-center gap-4"><button class="btn btn-primary" type="submit" :disabled="saving || loading">{{ saving ? t('lottery.saving') : t('lottery.save') }}</button><p v-if="saved" role="status" class="text-sm text-emerald-600 dark:text-emerald-400">{{ t('lottery.saved') }}</p></div>
          </form>
          <section class="card space-y-5 p-6 lg:col-span-2"><div class="flex items-center justify-between"><h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">{{ t('lottery.currentRound') }}</h2><span class="rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ snapshot.config.enabled ? t('lottery.inProgress') : t('lottery.paused') }}</span></div>
            <template v-if="snapshot.current"><p class="text-4xl font-bold tabular-nums text-gray-900 dark:text-gray-100">#{{ snapshot.current.id }}</p><dl class="space-y-4 text-sm text-gray-500"><div class="flex justify-between"><dt>{{ t('lottery.progress') }}</dt><dd class="font-medium text-gray-800 dark:text-gray-200">{{ snapshot.current.participant_count }} / {{ snapshot.current.participant_target }}</dd></div><div class="flex justify-between"><dt>{{ t('lottery.prizeAmount') }}</dt><dd>${{ snapshot.current.prize_amount.toFixed(2) }}</dd></div><div class="flex justify-between"><dt>{{ t('lottery.prizes') }}</dt><dd>{{ snapshot.current.winner_count }}</dd></div><div class="flex justify-between"><dt>{{ t('lottery.minRecharge') }}</dt><dd>${{ snapshot.current.min_recharge.toFixed(2) }}</dd></div></dl></template>
            <p v-else class="py-6 text-sm text-gray-500">{{ t('lottery.noRounds') }}</p>
            <router-link to="/lottery" class="inline-flex text-sm font-medium text-primary-600 hover:underline dark:text-primary-400">{{ t('lottery.openUserPage') }} →</router-link>
          </section>
        </div>
        <LotteryRoundsTable :rounds="snapshot.recent_rounds" :enabled="snapshot.config.enabled" />
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LotteryRoundsTable from '@/components/lottery/LotteryRoundsTable.vue'
import { lotteryAPI, type LotteryConfig, type LotterySnapshot } from '@/api/lottery'
const { t } = useI18n()
const snapshot = ref<LotterySnapshot | null>(null)
const form = reactive<LotteryConfig>({ enabled: false, prize_amount: 5, winner_count: 6, participant_target: 60, min_recharge: 50 })
const loading = ref(false); const saving = ref(false); const saved = ref(false); const error = ref('')
const budget = computed(() => (Number(form.prize_amount) * Number(form.winner_count) || 0).toFixed(2))
async function load() {
  loading.value = true
  try { snapshot.value = await lotteryAPI.adminGet(); Object.assign(form, snapshot.value.config); error.value = '' }
  catch { error.value = t('lottery.loadFailed') }
  finally { loading.value = false }
}
async function save() {
  if (saving.value) return
  saving.value = true; saved.value = false; error.value = ''
  try {
    await lotteryAPI.configure({ ...form }); saved.value = true
    await load()
  } catch (err: unknown) {
    error.value = (err as { reason?: string })?.reason === 'INVALID_LOTTERY_CONFIG' ? t('lottery.invalidConfig') : t('lottery.saveFailed')
  } finally { saving.value = false }
}
onMounted(load)
</script>
