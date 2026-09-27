<template>
  <AppLayout class="marketplace-layout">
    <div class="model-marketplace">
      <div class="marketplace-heading">
        <div>
          <h1>{{ t('modelPlaza.title') }}</h1>
          <p>{{ t('modelMarketplace.description') }}</p>
        </div>
        <div class="marketplace-actions">
          <label class="marketplace-search"
            ><Icon name="search" size="sm" /><input
              v-model="search"
              type="search"
              :placeholder="t('modelMarketplace.search')"
              :aria-label="t('modelMarketplace.search')"
          /></label>
          <button
            class="control-button"
            type="button"
            :aria-label="t('modelMarketplace.filters')"
            :aria-expanded="filtersOpen"
            aria-controls="marketplace-filters"
            :class="{ active: filtersOpen || filtersActive }"
            @click="filtersOpen = !filtersOpen"
          >
            <Icon name="filter" size="sm" /><span v-if="filtersActive" class="filter-dot"></span>
          </button>
          <button
            class="control-button"
            type="button"
            :aria-label="t('common.refresh')"
            :disabled="catalogLoading || monitorLoading"
            @click="refresh"
          >
            <Icon
              name="refresh"
              size="sm"
              :class="{ 'animate-spin': catalogLoading || monitorLoading }"
            />
          </button>
        </div>
      </div>

      <div v-if="filtersOpen" id="marketplace-filters" class="marketplace-filters">
        <label
          >{{ t('modelPlaza.filters.platformLabel')
          }}<select v-model="platform">
            <option value="all">{{ t('modelPlaza.filters.all') }}</option>
            <option v-for="item in platforms" :key="item" :value="item">
              {{ platformLabel(item) }}
            </option>
          </select></label
        >
        <label
          >{{ t('modelPlaza.filters.rateLabel')
          }}<select v-model="rate">
            <option value="all">{{ t('modelPlaza.filters.all') }}</option>
            <option v-for="item in rates" :key="item" :value="item">{{ item }}×</option>
          </select></label
        >
        <button v-if="filtersActive" type="button" class="reset-filters" @click="clearFilters">
          {{ t('modelMarketplace.reset') }}
        </button>
      </div>

      <div class="marketplace-toolbar">
        <div class="marketplace-legend" :aria-label="t('modelMarketplace.legend')">
          <span><i class="healthy"></i>{{ t('modelMarketplace.healthy') }}</span
          ><span><i class="warning"></i>{{ t('modelMarketplace.warning') }}</span
          ><span><i class="critical"></i>{{ t('modelMarketplace.critical') }}</span
          ><span><i class="unknown"></i>{{ t('modelMarketplace.unknown') }}</span>
        </div>
        <div class="range-controls" :aria-label="t('modelMarketplace.range')">
          <button
            v-for="item in ranges"
            :key="item"
            type="button"
            :aria-pressed="range === item"
            :class="{ selected: range === item }"
            :disabled="!monitorEnabled"
            @click="range = item"
          >
            {{ t(`channelMonitorV2.ranges.${item}`) }}
          </button>
        </div>
      </div>

      <div v-if="descriptionHtml" class="marketplace-description" v-html="descriptionHtml"></div>
      <div v-if="monitorError" class="marketplace-notice" role="status">
        <Icon name="exclamationCircle" size="sm" /><span>{{
          matrix ? t('modelMarketplace.staleStatus') : t('modelMarketplace.monitorFailed')
        }}</span
        ><button type="button" @click="loadMonitor()">{{ t('modelMarketplace.retry') }}</button>
      </div>
      <div v-else-if="!monitorEnabled" class="marketplace-notice" role="status">
        <Icon name="infoCircle" size="sm" />{{ t('modelMarketplace.monitorDisabledHint') }}
      </div>
      <div
        v-else-if="matrix && !matrix.coverage.coverage_complete"
        class="marketplace-notice"
        role="status"
      >
        <Icon name="clock" size="sm" />{{ t('modelMarketplace.partialHistory') }}
      </div>
      <div v-if="catalogError" class="marketplace-error" role="alert">
        <span>{{ t('modelPlaza.loadFailed') }}</span
        ><button type="button" @click="loadCatalog">{{ t('modelMarketplace.retry') }}</button>
      </div>
      <div
        v-if="catalogLoading && !catalog"
        class="marketplace-list"
        role="status"
        :aria-label="t('common.loading')"
      >
        <div v-for="n in 5" :key="n" class="group-skeleton"><i></i><i></i></div>
      </div>
      <div v-else-if="visibleGroups.length" class="marketplace-list">
        <MarketplaceGroupCard
          v-for="group in visibleGroups"
          :key="group.id"
          :group="group"
          :rows="rowsByGroup.get(group.id) ?? []"
          :coverage="matrix?.coverage"
          :loading="monitorLoading"
          :failed="monitorError"
          :monitor-enabled="monitorEnabled"
        />
      </div>
      <div v-else-if="!catalogError" class="marketplace-empty">
        <Icon name="search" size="lg" />
        <h2>
          {{ filtersActive || search ? t('modelPlaza.noSearchResult') : t('modelPlaza.empty') }}
        </h2>
        <button v-if="filtersActive || search" type="button" @click="clearFilters">
          {{ t('modelMarketplace.reset') }}
        </button>
      </div>
      <div v-if="catalog" class="marketplace-footer">
        <span>{{ t('modelMarketplace.groupCount', { count: visibleGroups.length }) }}</span
        ><span v-if="updatedAt">{{ t('modelMarketplace.updatedAt', { time: updatedAt }) }}</span
        ><span v-if="monitorEnabled" :title="t('modelMarketplace.availabilityHint')">{{
          t('modelMarketplace.statusHint')
        }}</span>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import MarketplaceGroupCard from '@/features/model-marketplace/MarketplaceGroupCard.vue'
import { useMarketplace } from '@/features/model-marketplace/useMarketplace'
import {
  effectiveGroupRate,
  filterMarketplaceGroups,
  indexMonitorRows,
} from '@/features/model-marketplace/marketplace'
import { platformLabel } from '@/utils/platformColors'
import type { MonitorRange } from '@/api/channelMonitorV2'
const { t, locale } = useI18n()
const {
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
} = useMarketplace()
const search = ref('')
const platform = ref('all')
const rate = ref<number | 'all'>('all')
const filtersOpen = ref(false)
const ranges: MonitorRange[] = ['90m', '24h', '7d', '30d']
const groups = computed(() => catalog.value?.groups ?? [])
const platforms = computed(() => [...new Set(groups.value.map((group) => group.platform))].sort())
const rates = computed(() =>
  [...new Set(groups.value.map(effectiveGroupRate))].sort((a, b) => a - b),
)
const filtersActive = computed(() => platform.value !== 'all' || rate.value !== 'all')
const visibleGroups = computed(() =>
  filterMarketplaceGroups(groups.value, search.value, platform.value, rate.value),
)
const rowsByGroup = computed(() => indexMonitorRows(matrix.value?.items ?? []))
const descriptionHtml = computed(() =>
  catalog.value?.description
    ? DOMPurify.sanitize(marked.parse(catalog.value.description) as string)
    : '',
)
const updatedAt = computed(() => {
  const time = Date.parse(matrix.value?.coverage.computed_at || '')
  return Number.isFinite(time)
    ? new Intl.DateTimeFormat(locale.value, {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      }).format(time)
    : ''
})
function clearFilters() {
  search.value = ''
  platform.value = 'all'
  rate.value = 'all'
}
watch(platforms, (values) => {
  if (platform.value !== 'all' && !values.includes(platform.value)) platform.value = 'all'
})
watch(rates, (values) => {
  if (rate.value !== 'all' && !values.includes(rate.value)) rate.value = 'all'
})
</script>

<style scoped>
.marketplace-layout { background: #fafbfc; }
.dark .marketplace-layout { background: #101722; }
.marketplace-layout :deep(.bg-mesh-gradient) { display: none; }

.model-marketplace {
  min-width: 0;
  color: #242a34;
}
.marketplace-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}
.marketplace-heading h1 {
  font-size: 25px;
  font-weight: 700;
  letter-spacing: -0.7px;
}
.marketplace-heading p {
  margin-top: 4px;
  color: #648098;
  font-size: 13px;
}
.marketplace-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.marketplace-search {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 340px;
  border: 1px solid #e6ebf2;
  border-radius: 7px;
  padding: 9px 12px;
  background: #fff;
  color: #8f9fb0;
}
.marketplace-search:focus-within {
  border-color: #38bdf8;
  box-shadow: 0 0 0 2px #38bdf815;
}
.marketplace-search input {
  width: 100%;
  min-width: 0;
  background: transparent;
  outline: 0;
  border: 0;
  padding: 0;
  font-size: 12px;
  color: #34465b;
  box-shadow: none;
}
.marketplace-search input::placeholder {
  color: #9cabbc;
}
.control-button {
  display: grid;
  place-items: center;
  position: relative;
  flex-shrink: 0;
  width: 35px;
  height: 35px;
  border: 1px solid #e6ebf2;
  border-radius: 7px;
  background: #fff;
  color: #55728b;
}
.control-button:hover,
.control-button.active {
  color: #0284c7;
  border-color: #b4deef;
  background: #f0faff;
}
.control-button:disabled {
  opacity: 0.5;
}
.filter-dot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #0ea5e9;
}
.marketplace-filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 15px;
  padding: 14px 16px;
  margin-bottom: 14px;
  background: #fff;
  border: 1px solid #e6ebf2;
  border-radius: 8px;
}
.marketplace-filters label {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: #718096;
}
.marketplace-filters select {
  min-width: 110px;
  border: 1px solid #e6ebf2;
  border-radius: 5px;
  font-size: 12px;
  padding: 5px 26px 5px 9px;
  background-color: transparent;
}
.reset-filters,
.marketplace-empty button {
  color: #0284c7;
  font-size: 12px;
}
.marketplace-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.marketplace-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 15px;
  font-size: 11px;
  color: #7e8b9c;
}
.marketplace-legend span {
  display: flex;
  align-items: center;
  gap: 5px;
}
.marketplace-legend i {
  width: 5px;
  height: 11px;
  border-radius: 2px;
  background: #08c58b;
}
.marketplace-legend i.warning {
  background: #f6bd16;
}
.marketplace-legend i.critical {
  background: #fb7185;
}
.marketplace-legend i.unknown {
  background: #cbd5e1;
}
.range-controls {
  display: flex;
  flex-shrink: 0;
  gap: 3px;
  padding: 3px;
  background: #edf1f6;
  border-radius: 6px;
}
.range-controls button {
  padding: 4px 10px;
  font-size: 11px;
  color: #728096;
  border-radius: 4px;
}
.range-controls button.selected {
  background: #fff;
  color: #2485ae;
  box-shadow: 0 1px 3px #14263608;
}
.range-controls button:disabled {
  opacity: 0.4;
}
.marketplace-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.marketplace-description {
  padding: 13px 16px;
  border: 1px solid #e6ebf2;
  border-radius: 8px;
  margin-bottom: 16px;
  font-size: 13px;
  line-height: 1.8;
  overflow-wrap: anywhere;
  background: #fff;
}
.marketplace-description :deep(a) {
  color: #0284c7;
  text-decoration: underline;
}
.marketplace-description :deep(ul) {
  list-style: disc;
  padding-left: 20px;
}
.marketplace-description :deep(ol) {
  list-style: decimal;
  padding-left: 20px;
}
.marketplace-description :deep(h1),
.marketplace-description :deep(h2),
.marketplace-description :deep(h3) {
  font-weight: 600;
}
.marketplace-notice,
.marketplace-error {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 12px 15px;
  margin-bottom: 16px;
  border-radius: 7px;
  background: #fff8eb;
  color: #997238;
  font-size: 12px;
}
.marketplace-notice button,
.marketplace-error button {
  text-decoration: underline;
  margin-left: auto;
  flex-shrink: 0;
}
.marketplace-error {
  background: #fff1f2;
  color: #be4561;
}
.group-skeleton {
  min-height: 117px;
  border: 1px solid #e6ebf2;
  border-radius: 9px;
  background: #fff;
  padding: 20px;
}
.group-skeleton i {
  display: block;
  width: 100px;
  height: 16px;
  background: #edf1f5;
  border-radius: 4px;
}
.group-skeleton i + i {
  margin-top: 18px;
  width: 230px;
  height: 24px;
}
.marketplace-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 15px;
  padding: 80px 24px;
  color: #8c9aaa;
  border: 1px dashed #dce3ec;
  border-radius: 9px;
}
.marketplace-empty h2 {
  font-size: 14px;
}
.marketplace-footer {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 20px;
  margin-top: 17px;
  color: #94a0b0;
  font-size: 11px;
}
.model-marketplace button:focus-visible,
.model-marketplace select:focus-visible {
  outline: 2px solid #0ea5e9;
  outline-offset: 3px;
}
.dark .model-marketplace {
  color: #e2e8f0;
}
.dark .marketplace-heading p {
  color: #9aaabe;
}
.dark .marketplace-search,
.dark .control-button,
.dark .marketplace-filters,
.dark .marketplace-description,
.dark .group-skeleton {
  background: #171e29;
  border-color: #2b3544;
}
.dark .marketplace-search input {
  color: #e2e8f0;
}
.dark .marketplace-filters select {
  border-color: #394659;
  color: #cbd5e1;
}
.dark .marketplace-filters select option {
  background: #171e29;
}
.dark .range-controls {
  background: #171e29;
}
.dark .range-controls button.selected {
  background: #2b3544;
  color: #7dd3fc;
}
.dark .marketplace-notice {
  background: #30291f;
  color: #d2b887;
}
.dark .marketplace-error {
  background: #36232b;
  color: #fda4af;
}
.dark .group-skeleton i {
  background: #2b3544;
}
@media (max-width: 1100px) {
  .marketplace-heading {
    align-items: flex-start;
  }
  .marketplace-search {
    width: 260px;
  }
}
@media (max-width: 640px) {
  .marketplace-heading {
    flex-direction: column;
    gap: 18px;
  }
  .marketplace-actions {
    width: 100%;
  }
  .marketplace-search {
    flex: 1;
    width: auto;
    min-width: 0;
  }
  .marketplace-search input {
    font-size: 16px;
  }
  .marketplace-toolbar {
    flex-direction: column-reverse;
    align-items: flex-start;
    gap: 14px;
  }
  .marketplace-notice {
    align-items: flex-start;
    line-height: 1.7;
  }
}
</style>
