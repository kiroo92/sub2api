<template>
  <div class="lucky-stage" :aria-busy="busy">
    <div v-if="mode === 'cards'" class="grid w-full max-w-md grid-cols-3 gap-3">
      <button v-for="card in 6" :key="card" class="lucky-card" :class="{ revealed: result && selected === card - 1 }" :disabled="busy || !canDraw" :aria-label="t('luckyDraw.pickCard', { number: card })" @click="emit('draw', card - 1)">
        <span v-if="result && selected === card - 1" class="text-sm font-semibold">{{ result.prize_name }}<span v-if="result.prize_amount > 0" class="mt-2 block text-2xl">${{ result.prize_amount.toFixed(2) }}</span></span>
        <span v-else class="text-4xl" aria-hidden="true">✦</span>
      </button>
    </div>
    <div v-else-if="mode === 'wheel'" class="relative">
      <div class="pointer" aria-hidden="true">▼</div>
      <div class="wheel" :style="{ background: wheelBackground, transform: `rotate(${rotation}deg)` }" aria-hidden="true">
        <span v-for="(prize, i) in prizes" :key="i" class="wheel-label" :style="labelStyle(i)">{{ prize.name }}</span>
      </div>
      <button class="wheel-button" :disabled="busy || !canDraw" @click="emit('draw', 0)">{{ busy ? '…' : t('luckyDraw.draw') }}</button>
    </div>
    <div v-else class="roller">
      <div class="text-5xl" aria-hidden="true">✦</div>
      <div class="roller-window mt-5" aria-hidden="true">
        <div class="roller-strip" :class="{ rolling: busy && !result }" :style="{ transform: `translateY(-${rollerOffset}px)`, '--cycle-height': `${prizes.length * 72}px` }">
          <div v-for="(prize, index) in rollerPrizes" :key="index" class="roller-item">{{ prize.name }}</div>
        </div>
      </div>
      <button class="btn mt-6 bg-white text-violet-700 hover:bg-violet-50" :disabled="busy || !canDraw" @click="emit('draw', 0)">{{ busy ? t('luckyDraw.drawing') : t('luckyDraw.draw') }}</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LuckyDrawPrize, LuckyDrawRecord } from '@/api/luckyDraw'
const props = defineProps<{ mode: 'cards' | 'wheel' | 'roller'; prizes: LuckyDrawPrize[]; busy: boolean; canDraw: boolean; result: LuckyDrawRecord | null; selected: number }>()
const emit = defineEmits<{ draw: [card: number] }>()
const { t } = useI18n()
const rotation = ref(0)
const rollerOffset = ref(0)
const rollerPrizes = computed(() => Array.from({ length: 7 }, () => props.prizes).flat())
const colors = ['#7c3aed', '#a78bfa', '#5b21b6', '#c4b5fd', '#8b5cf6', '#6d28d9']
const wheelBackground = computed(() => `conic-gradient(${props.prizes.map((_, i) => `${colors[i % colors.length]} ${i * 360 / props.prizes.length}deg ${(i + 1) * 360 / props.prizes.length}deg`).join(',')})`)
const labelStyle = (i: number) => ({ transform: `translate(-50%, -50%) rotate(${(i + 0.5) * 360 / props.prizes.length}deg) translateY(-106px) rotate(-${(i + 0.5) * 360 / props.prizes.length}deg)` })
watch(() => props.result, result => {
  if (!result) { rollerOffset.value = 0; return }
  const offset = (result.prize_index + 0.5) * 360 / props.prizes.length
  rotation.value = (Math.ceil(rotation.value / 360) + 5) * 360 - offset
  rollerOffset.value = (props.prizes.length * 5 + result.prize_index) * 72
})
</script>

<style scoped>
.lucky-stage { min-height: 360px; display: flex; align-items: center; justify-content: center; padding: 24px 12px; }
.lucky-card { min-height: 136px; border-radius: 18px; background: linear-gradient(145deg, #8b5cf6, #4c1d95); color: white; border: 3px solid #c4b5fd; box-shadow: 0 8px 20px #6d28d920; transition: transform .4s, background .4s; padding: 10px; overflow-wrap: anywhere; }
.lucky-card:not(:disabled):hover { transform: translateY(-5px); }
.lucky-card:disabled { cursor: default; }
.lucky-card.revealed { background: linear-gradient(145deg, #fef3c7, #fbbf24); color: #78350f; transform: rotateY(360deg); border-color: #f59e0b; }
.wheel { width: 300px; height: 300px; border-radius: 50%; border: 8px solid #ede9fe; position: relative; transition: transform 1.8s cubic-bezier(.15,.7,.15,1); box-shadow: 0 8px 30px #6d28d930; }
.wheel-label { position: absolute; top: 50%; left: 50%; width: 82px; text-align: center; color: white; font-size: 12px; font-weight: 700; overflow-wrap: anywhere; }
.pointer { position: absolute; top: -18px; left: calc(50% - 14px); z-index: 2; color: #f59e0b; font-size: 32px; }
.wheel-button { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); width: 76px; height: 76px; background: white; color: #6d28d9; border-radius: 50%; font-weight: 700; box-shadow: 0 2px 10px #0002; }
.roller { width: 100%; max-width: 360px; min-height: 280px; border-radius: 24px; background: linear-gradient(145deg, #8b5cf6, #4c1d95); color: white; text-align: center; padding: 38px 20px; border: 6px solid #ddd6fe; }
.roller-window { height: 72px; overflow: hidden; border-radius: 12px; background: #ffffff15; }
.roller-strip { transition: transform 1.8s cubic-bezier(.15,.7,.15,1); }
.roller-item { height: 72px; display: flex; justify-content: center; align-items: center; padding: 0 12px; font-size: 24px; font-weight: 700; }
.rolling { animation: roll .3s linear infinite; }
@keyframes roll { to { transform: translateY(calc(-1 * var(--cycle-height))); } }
@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation: none !important; transition: none !important; } }
@media (max-width: 380px) { .wheel { width: 280px; height: 280px; } .lucky-card { min-height: 118px; } }
</style>
