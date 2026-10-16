<template>
  <Teleport to="body">
    <div class="draw-modal-backdrop" @click.self="emit('close')">
      <section ref="dialog" class="draw-result-modal" :class="[rarity, { 'no-motion': skipAnimation }]" role="dialog" aria-modal="true" aria-labelledby="draw-result-title" tabindex="-1" @keydown="keyDown">
        <span class="modal-rarity">{{ t(`luckyDraw.rarity.${rarity}`) }}</span>
        <div class="modal-gift" aria-hidden="true"><svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M4 10h16v10H4zM3 6h18v4H3zM12 6v14" /><path d="M12 6H8.5A2.5 2.5 0 1 1 11 3.5L12 6Zm0 0h3.5A2.5 2.5 0 1 0 13 3.5L12 6Z" /></svg></div>
        <h2 id="draw-result-title" class="mt-5 text-xl font-bold">{{ demo ? t('luckyDraw.demoTimes', { count: records.length }) : t('luckyDraw.drawTimes', { count: records.length }) }}</h2>
        <p class="modal-total" aria-live="polite">¥{{ total.toFixed(2) }}</p>
        <p class="modal-receipt">{{ demo ? t('luckyDraw.demoComplete', { count: records.length }) : t('luckyDraw.batchComplete', { count: records.length, amount: total.toFixed(2) }) }}</p>
        <div class="modal-results"><div v-for="(record, i) in records" :key="record.id + ':' + i" class="modal-result" :class="luckyDrawRarity(record.prize_amount)"><span class="modal-index">#{{ i + 1 }}</span><strong>{{ record.prize_amount > 0 ? '¥' + record.prize_amount : t('luckyDraw.noPrize') }}</strong><span v-if="record.guaranteed" class="ml-auto text-[10px]">{{ t('luckyDraw.guarantee') }}</span></div></div>
        <p class="mt-4 text-[11px] text-white/40">{{ t('luckyDraw.resultHint') }}</p>
        <div class="modal-actions"><button class="modal-again" :disabled="!canAgain" @click="emit('again')">{{ demo ? t('luckyDraw.demoTimes', { count: records.length }) : t('luckyDraw.drawTimes', { count: records.length }) }}</button><button @click="emit('close')">{{ t('luckyDraw.close') }}</button></div>
      </section>
    </div>
  </Teleport>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LuckyDrawRecord } from '@/api/luckyDraw'
import { luckyDrawRarity } from '@/utils/luckyDraw'
const props = defineProps<{ records: LuckyDrawRecord[]; demo: boolean; canAgain: boolean; skipAnimation?: boolean }>()
const emit = defineEmits<{ close: []; again: [] }>()
const { t } = useI18n()
const dialog = ref<HTMLElement | null>(null)
const total = computed(() => props.records.reduce((sum, r) => sum + Math.round(r.prize_amount * 100), 0) / 100)
const rarity = computed(() => luckyDrawRarity(Math.max(0, ...props.records.map(r => r.prize_amount))))
let previousFocus: HTMLElement | null = null
let previousOverflow = ''
onMounted(() => { previousFocus = document.activeElement as HTMLElement; previousOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'; dialog.value?.focus() })
onUnmounted(() => { document.body.style.overflow = previousOverflow; previousFocus?.focus() })
function keyDown(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.preventDefault(); emit('close') }
  if (event.key !== 'Tab') return
  const buttons = dialog.value?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')
  if (!buttons?.length) return
  const first = buttons[0]!, last = buttons[buttons.length - 1]!
  if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && (document.activeElement === last || document.activeElement === dialog.value)) { event.preventDefault(); first.focus() }
}
</script>
<style scoped>
.draw-modal-backdrop { position: fixed; inset: 0; z-index: 9999; background: #06090ed1; backdrop-filter: blur(5px); display: flex; justify-content: center; align-items: center; padding: 20px; overflow-y: auto; }
.draw-result-modal { --result-color: #81c4a3; position: relative; width: 100%; max-width: 430px; max-height: calc(100dvh - 40px); overflow-y: auto; padding: 30px 26px 24px; border: 1px solid color-mix(in srgb,var(--result-color) 40%,#41454b); border-radius: 26px; color: #f6f6f6; background: radial-gradient(ellipse at 50% 0%,#ffffff12,transparent 55%),linear-gradient(160deg,#252a2d,#090d15 85%); box-shadow: 0 0 70px color-mix(in srgb,var(--result-color) 18%,transparent),0 30px 80px #0008; text-align: center; animation: modal-arrive .35s ease-out; }
.draw-result-modal.legendary { --result-color: #f0d98e; }.draw-result-modal.epic { --result-color: #bea2ef; }.draw-result-modal.rare { --result-color: #7bc5ef; }.draw-result-modal.none { --result-color: #a2a5ae; }
.modal-rarity { display: inline-block; border-radius: 999px; padding: 3px 12px; font-size: 11px; font-weight: 700; color: var(--result-color); background: color-mix(in srgb,var(--result-color) 20%,transparent); }
.modal-gift { display: grid; place-items: center; width: 78px; height: 78px; margin: 13px auto 0; border-radius: 50%; font-size: 34px; color: #15191b; background: radial-gradient(circle at 35% 25%,#fff6cf,var(--result-color)); box-shadow: 0 0 28px color-mix(in srgb,var(--result-color) 40%,transparent); }
.modal-total { margin: 18px 0; color: var(--result-color); font-size: 46px; line-height: 1.1; font-variant-numeric: tabular-nums; letter-spacing: -.04em; }
.modal-receipt { font-size: 12px; line-height: 1.7; color: #aeb0b6; }
.modal-results { display: grid; grid-template-columns: repeat(3,minmax(0,1fr)); gap: 8px; margin-top: 16px; text-align: left; }
.modal-result { --item-color: #90bca5; display: flex; align-items: center; gap: 5px; min-width: 0; border: 1px solid color-mix(in srgb,var(--item-color) 40%,#45484f); border-radius: 11px; padding: 9px 8px; font-size: 12px; background: color-mix(in srgb,var(--item-color) 8%,transparent); color: var(--item-color); }
.modal-result.rare { --item-color: #81c7f0; }.modal-result.epic { --item-color: #bca2f0; }.modal-result.legendary { --item-color: #f0d98e; box-shadow: 0 0 18px #f0d98e16; }.modal-result.none { --item-color: #92959e; }
.modal-index { color: #ffffff55; font-size: 10px; }.modal-result strong { overflow-wrap: anywhere; }
.modal-actions { display: flex; gap: 10px; margin-top: 20px; }.modal-actions button { flex: 1; border-radius: 12px; border: 1px solid #ffffff26; padding: 12px 8px; font-size: 13px; font-weight: 700; background: #ffffff08; }.modal-actions .modal-again { color: #43300d; border-color: #e0b45e; background: linear-gradient(135deg,#f6e3a3,#d7ac5b); box-shadow: 0 6px 20px #d7ac5b20; }.modal-actions button:disabled { opacity: .4; }
@keyframes modal-arrive { from { opacity: 0; transform: translateY(14px) scale(.97); } to { opacity: 1; transform: none; } }
.no-motion { animation: none; }
@media (max-width: 420px) { .draw-result-modal { padding: 25px 16px 20px; }.modal-results { grid-template-columns: repeat(2,minmax(0,1fr)); } }
@media (prefers-reduced-motion: reduce) { .draw-result-modal { animation: none; } }
</style>
