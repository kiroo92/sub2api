<template>
  <div class="reward-reveal" :class="[rarity, { compact }]" :data-rarity="rarity">
    <div v-if="rarity !== 'none' && rarity !== 'common'" class="reward-sparks" aria-hidden="true"><i v-for="n in 12" :key="n" :style="{ '--angle': `${n * 30}deg`, '--delay': `${n * 0.035}s` }">✦</i></div>
    <span class="rarity-badge">{{ t(`luckyDraw.rarity.${rarity}`) }}</span>
    <div class="mt-2 text-xl font-bold">{{ record.prize_name }}</div>
    <p class="mt-2 text-sm">{{ demo ? t('luckyDraw.demoReward') : record.prize_amount > 0 ? t('luckyDraw.credited', { amount: record.prize_amount.toFixed(2) }) : t('luckyDraw.noWin') }}</p>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LuckyDrawRecord } from '@/api/luckyDraw'
import { luckyDrawRarity } from '@/utils/luckyDraw'
const props = defineProps<{ record: LuckyDrawRecord; demo?: boolean; compact?: boolean }>()
const { t } = useI18n()
const rarity = computed(() => luckyDrawRarity(props.record.prize_amount))
</script>
<style scoped>
.reward-reveal { --reward-color: #10b981; position: relative; isolation: isolate; overflow: hidden; border: 1px solid color-mix(in srgb, var(--reward-color) 40%, transparent); border-radius: 18px; padding: 24px; color: var(--reward-color); background: color-mix(in srgb, var(--reward-color) 8%, transparent); animation: appear .5s ease-out; }
.rare { --reward-color: #3b82f6; }
.epic { --reward-color: #a855f7; box-shadow: inset 0 0 30px #a855f720; }
.legendary { --reward-color: #f59e0b; box-shadow: inset 0 0 40px #f59e0b30, 0 0 18px #f59e0b20; }
.none { --reward-color: #6b7280; }
.compact { padding: 12px 8px; text-align: center; }
.compact .rarity-badge { font-size: 10px; padding: 1px 6px; }
.compact p { font-size: 10px; }
.rarity-badge { display: inline-block; border-radius: 999px; border: 1px solid currentColor; padding: 2px 12px; font-size: 12px; font-weight: 700; }
.reward-sparks { position: absolute; inset: 0; z-index: -1; pointer-events: none; }
.reward-sparks i { position: absolute; top: 50%; left: 50%; font-style: normal; animation: sparkle 1.2s var(--delay) ease-out both; }
@keyframes appear { from { opacity: 0; transform: scale(.94); } to { opacity: 1; transform: scale(1); } }
@keyframes sparkle { from { opacity: 0; transform: rotate(var(--angle)) translateX(12px) scale(.3); } 30% { opacity: .8; } to { opacity: 0; transform: rotate(var(--angle)) translateX(160px) scale(1.2); } }
@media (prefers-reduced-motion: reduce) { .reward-reveal, .reward-sparks i { animation: none; } .reward-sparks { display: none; } }
</style>
