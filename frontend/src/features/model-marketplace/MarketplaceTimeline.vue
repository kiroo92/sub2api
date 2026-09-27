<template>
  <div class="market-timeline">
    <div class="timeline-summary">
      <span v-if="label" class="timeline-platform">{{ label }}</span>
      <div
        class="timeline-bars"
        :style="{ gap: slots.length > 120 ? '1px' : '2px' }"
        :aria-label="t('modelMarketplace.history')"
        @mouseleave="hovered = null"
      >
        <button
          v-for="(slot, index) in slots"
          :key="slot.start"
          type="button"
          class="timeline-bar"
          :class="barState(slot.bucket)"
          :tabindex="index === focusIndex ? 0 : -1"
          :aria-label="tooltip(slot)"
          :title="tooltip(slot)"
          @mouseenter="showTooltip(slot, $event)"
          @focus="focusSlot(index, slot, $event)"
          @blur="hovered = null"
          @keydown="moveFocus($event, index)"
        ></button>
        <span v-if="!slots.length" class="timeline-empty">{{ t('modelMarketplace.noData') }}</span>
      </div>
      <strong :title="t('modelMarketplace.availabilityHint')">{{
        formatAvailability(availability(row.metrics, row.health))
      }}</strong>
    </div>
    <Teleport to="body">
      <div v-if="hovered" role="tooltip" class="market-timeline-tooltip" :style="tooltipPosition">
        {{ tooltip(hovered) }}
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorMatrixBucket, MonitorMatrixRow, MonitorCoverage } from '@/api/channelMonitorV2'
import { formatMonitorMs, formatMonitorPercent } from '@/features/channel-monitor-v2/monitorFormat'
import { alignTimeline, availability, type TimelineSlot } from './marketplace'
const props = defineProps<{ row: MonitorMatrixRow; coverage: MonitorCoverage; label?: string }>()
const { t, locale } = useI18n()
const slots = computed(() => alignTimeline(props.row.buckets, props.coverage))
const hovered = ref<TimelineSlot | null>(null)
const focusIndex = ref(0)
const tooltipPosition = ref({ left: '0px', top: '0px' })
function formatAvailability(value: number | null) {
  return value == null
    ? '—'
    : `${(value * 100).toLocaleString(locale.value, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}%`
}
function barState(bucket?: MonitorMatrixBucket) {
  return bucket && availability(bucket.metrics, bucket.health) != null
    ? bucket.health.overall
    : 'unknown'
}
function tooltip(slot: TimelineSlot) {
  const formatter = new Intl.DateTimeFormat(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
  const time = `${formatter.format(slot.start)} – ${formatter.format(slot.end)}`
  if (!slot.bucket || availability(slot.bucket.metrics, slot.bucket.health) == null)
    return `${time}\n${t('modelMarketplace.noData')}`
  return `${time}\n${t('modelMarketplace.availability')}: ${formatAvailability(availability(slot.bucket.metrics, slot.bucket.health))}\n${t('channelMonitorV2.metrics.ttft')}: ${formatMonitorMs(slot.bucket.metrics.ttft.p50_ms)}\n${t('channelMonitorV2.metrics.cacheRate')}: ${formatMonitorPercent(slot.bucket.metrics.cache_rate, locale.value)}`
}
function showTooltip(slot: TimelineSlot, event: Event) {
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  tooltipPosition.value = {
    left: `${Math.min(Math.max(12, rect.left - 110), window.innerWidth - 272)}px`,
    top: `${Math.min(rect.bottom + 9, window.innerHeight - 140)}px`,
  }
  hovered.value = slot
}
function focusSlot(index: number, slot: TimelineSlot, event: Event) {
  focusIndex.value = index
  showTooltip(slot, event)
}
function moveFocus(event: KeyboardEvent, index: number) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? slots.value.length - 1
        : Math.max(
            0,
            Math.min(slots.value.length - 1, index + (event.key === 'ArrowRight' ? 1 : -1)),
          )
  const buttons = (event.currentTarget as HTMLElement).parentElement?.querySelectorAll('button')
  ;(buttons?.[next] as HTMLButtonElement | undefined)?.focus()
}
</script>

<style scoped>
.timeline-summary {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.timeline-platform {
  font-size: 11px;
  color: #718096;
  min-width: 48px;
}
.timeline-bars {
  display: flex;
  align-items: center;
  height: 24px;
  flex: 1;
  min-width: 0;
}
.timeline-bar {
  flex: 1 1 0;
  min-width: 0;
  height: 23px;
  padding: 0;
  border: 0;
  border-radius: 2px;
  background: transparent;
  color: #08c58b;
  position: relative;
  transition:
    opacity 0.15s,
    transform 0.15s;
}
.timeline-bar::before {
  content: '';
  position: absolute;
  inset: 0;
  background: currentColor;
  mask-image: repeating-linear-gradient(90deg, #000 0 3px, transparent 3px 5px);
  border-radius: 1px;
}
.timeline-bar:hover,
.timeline-bar:focus-visible {
  transform: scaleY(1.18);
  outline: 2px solid #38bdf8;
  outline-offset: 2px;
  z-index: 1;
}
.timeline-bar.warning {
  color: #f6bd16;
}
.timeline-bar.critical {
  color: #fb7185;
}
.timeline-bar.unknown {
  color: #e2e8f0;
}
.dark .timeline-bar.unknown {
  color: #3b4655;
}
.timeline-summary strong {
  width: 70px;
  flex-shrink: 0;
  font-size: 15px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  text-align: right;
}
.timeline-empty {
  color: #94a3b8;
  font-size: 12px;
}
.market-timeline-tooltip {
  position: fixed;
  width: 260px;
  max-width: calc(100vw - 24px);
  z-index: 100;
  border-radius: 7px;
  padding: 11px 13px;
  background: #17212fee;
  color: #f8fafc;
  font-size: 11px;
  line-height: 1.8;
  white-space: pre-line;
  pointer-events: none;
  box-shadow: 0 6px 24px #0f172a20;
}
@media (prefers-reduced-motion: reduce) {
  .timeline-bar {
    transition: none;
  }
}
</style>
