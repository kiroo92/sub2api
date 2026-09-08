<template>
  <section class="card overflow-hidden" aria-labelledby="lottery-rounds-title">
    <div class="flex items-center justify-between border-b border-gray-100 px-6 py-5 dark:border-dark-700">
      <h2 id="lottery-rounds-title" class="text-lg font-semibold text-gray-900 dark:text-gray-100">{{ t('lottery.recentRounds') }}</h2>
      <span class="text-xs text-gray-500">{{ t('lottery.latestTwenty') }}</span>
    </div>
    <div v-if="rounds.length" class="overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr>
          <th class="px-6 py-3 font-medium">{{ t('lottery.round') }}</th>
          <th class="px-6 py-3 font-medium">{{ t('lottery.participants') }}</th>
          <th class="px-6 py-3 font-medium">{{ t('lottery.winnersCount') }}</th>
          <th class="px-6 py-3 font-medium">{{ t('lottery.status') }}</th>
        </tr></thead>
        <tbody class="divide-y divide-gray-100 text-gray-600 dark:divide-dark-700 dark:text-gray-300">
          <tr v-for="round in rounds" :key="round.id">
            <td class="px-6 py-4 font-medium tabular-nums">#{{ round.id }}</td>
            <td class="px-6 py-4 tabular-nums">{{ round.participant_count }} / {{ round.participant_target }}</td>
            <td class="px-6 py-4 tabular-nums">{{ round.winners_drawn }}</td>
            <td class="whitespace-nowrap px-6 py-4"><span :class="round.status === 'open' && enabled ? 'text-primary-600 dark:text-primary-400' : ''">{{ round.status === 'drawn' ? t('lottery.drawn') : enabled ? t('lottery.inProgress') : t('lottery.paused') }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-else class="px-6 py-12 text-center text-sm text-gray-500">{{ t('lottery.noRounds') }}</p>
  </section>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { LotteryRound } from '@/api/lottery'
defineProps<{ rounds: LotteryRound[]; enabled: boolean }>()
const { t } = useI18n()
</script>
