<template>
  <article class="market-group" :aria-labelledby="`market-group-${group.id}`">
    <div class="group-overview">
      <div class="group-information">
        <div class="group-badges">
          <span class="platform-badge" :class="platformBadgeLightClass(group.platform)"
            ><PlatformIcon :platform="group.platform as GroupPlatform" size="sm" />{{
              platformLabel(group.platform)
            }}</span
          >
          <span
            class="rate-badge"
            :class="{
              'custom-rate':
                group.user_rate_multiplier != null &&
                group.user_rate_multiplier !== group.rate_multiplier,
            }"
          >
            {{
              group.user_rate_multiplier != null &&
              group.user_rate_multiplier !== group.rate_multiplier
                ? t('modelMarketplace.yourRate')
                : t('modelMarketplace.groupRate')
            }}
            ×{{ effectiveGroupRate(group) }}
          </span>
          <span v-if="group.image_rate_independent" class="image-badge"
            >{{ t('modelMarketplace.imageRate') }} ×{{ group.image_rate_multiplier }}</span
          >
          <span v-if="group.subscription_type === 'subscription'" class="rate-badge">{{
            t('modelPlaza.badges.subscription')
          }}</span>
          <span v-if="group.is_exclusive" class="rate-badge">{{
            t('modelPlaza.badges.exclusive')
          }}</span>
        </div>
        <div class="group-heading">
          <span class="group-icon"
            ><PlatformIcon :platform="group.platform as GroupPlatform" size="lg"
          /></span>
          <div>
            <h2 :id="`market-group-${group.id}`">{{ group.name }}</h2>
            <p v-if="group.description">{{ group.description }}</p>
          </div>
        </div>
      </div>
      <div class="group-status">
        <template v-if="coverage && rows.length">
          <MarketplaceTimeline
            v-for="row in rows"
            :key="row.platform"
            :row="row"
            :coverage="coverage"
            :label="rows.length > 1 ? platformLabel(row.platform) : undefined"
          />
        </template>
        <div v-else class="status-placeholder" :class="{ 'is-loading': loading }">
          <span class="empty-timeline" aria-hidden="true"></span>
          <span>{{
            loading
              ? t('common.loading')
              : !monitorEnabled
                ? t('modelMarketplace.monitorDisabled')
                : failed
                  ? t('modelMarketplace.statusUnavailable')
                  : t('modelMarketplace.noData')
          }}</span>
        </div>
        <button
          type="button"
          class="pricing-toggle"
          :aria-expanded="expanded"
          :aria-controls="`market-pricing-${group.id}`"
          @click="expanded = !expanded"
        >
          {{ expanded ? t('modelMarketplace.hidePricing') : t('modelMarketplace.showPricing') }}
          <Icon name="chevronDown" size="sm" :class="{ 'rotate-180': expanded }" />
        </button>
      </div>
    </div>
    <div
      v-if="expanded"
      :id="`market-pricing-${group.id}`"
      class="group-pricing"
      role="region"
      :aria-label="t('modelMarketplace.pricingFor', { group: group.name })"
    >
      <div class="pricing-heading">
        <span>{{ t('modelMarketplace.modelCount', { count: group.models.length }) }}</span
        ><span>{{ t('modelMarketplace.pricingHint') }}</span>
      </div>
      <PlazaGroupPricing :group="group" />
    </div>
  </article>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PlazaGroupPricing from '@/components/modelPlaza/PlazaGroupPricing.vue'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import type { MonitorCoverage, MonitorMatrixRow } from '@/api/channelMonitorV2'
import type { GroupPlatform } from '@/types'
import { platformBadgeLightClass, platformLabel } from '@/utils/platformColors'
import MarketplaceTimeline from './MarketplaceTimeline.vue'
import { effectiveGroupRate } from './marketplace'
defineProps<{
  group: ModelPlazaGroup
  rows: MonitorMatrixRow[]
  coverage?: MonitorCoverage
  loading: boolean
  failed: boolean
  monitorEnabled: boolean
}>()
const { t } = useI18n()
const expanded = ref(false)
</script>

<style scoped>
.market-group {
  border: 1px solid #e7ebf0;
  border-radius: 9px;
  background: #fff;
  overflow: hidden;
}
.group-overview {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(250px, 42%);
  align-items: center;
  gap: 30px;
  padding: 17px 20px;
  min-height: 115px;
}
.group-information {
  min-width: 0;
}
.group-badges {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 13px;
}
.group-badges > span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  line-height: 1.6;
  padding: 2px 10px;
  border-radius: 20px;
  font-weight: 500;
}
.rate-badge {
  border: 1px solid #e8e9ee;
  background: #f5f5f7;
  color: #52545e;
}
.rate-badge.custom-rate {
  background: #eff6ff;
  color: #2563eb;
  border-color: #dbeafe;
}
.image-badge {
  border: 1px solid #f5d8ff;
  background: #fdf4ff;
  color: #b52ad0;
}
.group-heading {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.group-icon {
  display: grid;
  place-items: center;
  width: 43px;
  height: 43px;
  flex-shrink: 0;
  border: 1px solid #e9edf2;
  border-radius: 7px;
  color: #262a32;
}
.group-heading > div {
  min-width: 0;
}
.group-heading h2 {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.55;
  color: #181b21;
  overflow-wrap: anywhere;
}
.group-heading p {
  margin-top: 4px;
  font-size: 12px;
  color: #7a8190;
  line-height: 1.65;
  overflow-wrap: anywhere;
  white-space: pre-line;
}
.group-status {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}
.pricing-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  align-self: flex-end;
  border: 1px solid #e8ecf2;
  border-radius: 6px;
  padding: 7px 13px;
  font-size: 12px;
  color: #55728b;
  transition:
    border-color 0.15s,
    background 0.15s;
}
.pricing-toggle:hover {
  border-color: #a7dcf1;
  background: #f2faff;
}
.pricing-toggle:focus-visible {
  outline: 2px solid #0ea5e9;
  outline-offset: 3px;
}
.status-placeholder {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 24px;
  font-size: 11px;
  color: #8791a0;
}
.empty-timeline {
  height: 22px;
  flex: 1;
  background: repeating-linear-gradient(90deg, #e7ecf1 0 3px, transparent 3px 5px);
}
.is-loading .empty-timeline {
  opacity: 0.55;
}
.group-pricing {
  border-top: 1px solid #e7ebf0;
}
.pricing-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 13px 20px;
  font-size: 11px;
  color: #7a8190;
  background: #fafbfd;
  border-bottom: 1px solid #edf0f4;
}
.dark .market-group {
  background: #171e29;
  border-color: #2b3544;
}
.dark .group-heading h2 {
  color: #edf2f7;
}
.dark .group-heading p {
  color: #9aa6b7;
}
.dark .group-icon {
  color: #e2e8f0;
  border-color: #344050;
}
.dark .rate-badge {
  background: #273140;
  border-color: #344050;
  color: #cbd5e1;
}
.dark .rate-badge.custom-rate {
  color: #7dd3fc;
}
.dark .image-badge {
  background: #3b2142;
  border-color: #563260;
  color: #e9a6f9;
}
.dark .pricing-toggle {
  border-color: #334155;
  color: #b0c4dc;
}
.dark .pricing-toggle:hover {
  background: #203245;
}
.dark .empty-timeline {
  background: repeating-linear-gradient(90deg, #344050 0 3px, transparent 3px 5px);
}
.dark .group-pricing,
.dark .pricing-heading {
  border-color: #2b3544;
}
.dark .pricing-heading {
  background: #131a25;
  color: #9aa6b7;
}
@media (max-width: 900px) {
  .group-overview {
    grid-template-columns: 1fr;
    gap: 17px;
    padding: 17px;
  }
  .group-status {
    gap: 14px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .pricing-toggle {
    transition: none;
  }
}
</style>
