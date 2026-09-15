<template>
  <div class="package-detail">
    <div class="package-detail__grid">
      <section class="package-detail__product" aria-labelledby="package-title">
        <header>
          <div class="package-detail__eyebrow">
            <Icon name="gift" size="sm" aria-hidden="true" />
            <span>{{ t('packages.detail') }}</span>
            <span class="package-detail__pill">{{ t(group.plan.validity_days === 7 ? 'packages.week' : 'packages.month') }}</span>
          </div>
          <h1 id="package-title" class="package-detail__title">{{ group.plan.name }}</h1>
          <p class="package-detail__subtitle">{{ group.plan.description || t('packages.instantSummary') }}</p>
        </header>

        <div class="package-detail__visual">
          <img class="package-detail__art" src="@/assets/packages/group-card.png" alt="" aria-hidden="true" />
        </div>

        <dl class="package-detail__facts">
          <div>
            <dt>{{ t('packages.baseQuota') }}</dt>
            <dd>{{ quota(group.plan.base_quota_usd) }} <small>USD</small></dd>
          </div>
          <div>
            <dt>{{ t('packages.eachPeriodQuota') }}</dt>
            <dd>{{ quota(periodQuota) }} <small>USD</small></dd>
          </div>
          <div>
            <dt>{{ t('packages.validityLabel') }}</dt>
            <dd>{{ group.plan.validity_days }} <small>{{ t('packages.days') }}</small></dd>
          </div>
        </dl>
        <div class="package-detail__benefits">
          <span><Icon name="bolt" size="sm" aria-hidden="true" />{{ t('packages.instantTitle') }}</span>
          <span><Icon name="gift" size="sm" aria-hidden="true" />{{ t('packages.settlementTitle') }}</span>
          <span><Icon name="users" size="sm" aria-hidden="true" />{{ t('packages.multiGroupTitle') }}</span>
        </div>
      </section>

      <aside class="package-detail__rail">
        <section class="package-detail__group" aria-labelledby="package-group-target">
          <div class="package-detail__heading">
            <h2 id="package-group-target">{{ t('packages.target') }}</h2>
            <span class="package-detail__status">{{ t(`packages.${displayStatus}`) }}</span>
          </div>
          <div class="package-detail__progress-label">
            <span>{{ t('packages.groupNumber', { id: group.id }) }}</span>
            <strong>{{ t('packages.members', { count: memberCount, target: group.target_members }) }}</strong>
          </div>
          <progress :aria-label="t('packages.target')" :value="memberCount" :max="Math.max(1, group.target_members)" />

          <div class="package-detail__tiers">
            <div
              v-for="tier in tiers"
              :key="tier.members"
              class="package-detail__tier"
              :class="{ 'is-current': reachedTier?.members === tier.members }"
            >
              <div class="package-detail__tier-heading">
                <strong>{{ t('packages.tier', { members: tier.members }) }}</strong>
                <Icon v-if="reachedTier?.members === tier.members" name="checkCircle" size="sm" :aria-label="t('packages.reachedTier')" />
              </div>
              <b>{{ quota(tier.quota_usd) }}</b>
              <span>{{ t('packages.totalQuotaUnit') }}</span>
            </div>
          </div>

          <div class="package-detail__quota-summary">
            <span>{{ t(group.status === 'settled' ? 'packages.finalQuota' : 'packages.expectedQuota') }}</span>
            <strong>{{ quota(reachedQuota) }} <small>USD</small></strong>
          </div>
          <div v-if="group.status !== 'settled'" class="package-detail__countdown">
            <div>
              <span>{{ t(group.status === 'draft' ? 'packages.draftDuration' : 'packages.timeLeft') }}</span>
              <small v-if="group.status === 'draft'">{{ t('packages.startsAfterPayment') }}</small>
              <small v-else-if="group.ends_at">{{ formatDateTimeToMinute(group.ends_at) }}</small>
            </div>
            <strong class="package-detail__timer">{{ countdown }}</strong>
          </div>
        </section>

        <section class="package-detail__rules" aria-labelledby="package-rules-title">
          <h2 id="package-rules-title">{{ t('packages.rules') }}</h2>
          <ul>
            <li v-for="rule in rules" :key="rule.title" class="package-detail__rule">
              <span class="package-detail__rule-icon" :class="`package-detail__rule-icon--${rule.tone}`">
                <Icon :name="rule.icon" size="sm" aria-hidden="true" />
              </span>
              <div><h3>{{ t(rule.title) }}</h3><p>{{ rule.text }}</p></div>
            </li>
          </ul>
        </section>
      </aside>
    </div>

    <footer class="package-detail__action">
      <template v-if="group.joined">
        <div class="package-detail__purchased" role="status">
          <Icon name="checkCircle" size="lg" aria-hidden="true" />
          <div>
            <strong>{{ t('packages.baseDelivered') }}</strong>
            <p>{{ t(group.status === 'settled' ? 'packages.rewardSettled' : 'packages.rewardPending') }}</p>
          </div>
        </div>
        <div class="package-detail__links">
          <RouterLink v-if="group.order_id" to="/orders" class="package-detail__order-link">{{ t('packages.resume') }}</RouterLink>
          <RouterLink to="/dashboard#subscriptions" class="btn btn-primary">
            {{ t('packages.viewSubscriptions') }}<Icon name="arrowRight" size="sm" aria-hidden="true" />
          </RouterLink>
        </div>
      </template>
      <template v-else>
        <div class="package-detail__price">
          <span>{{ t('packages.price') }}</span>
          <strong>{{ formatPaymentAmount(group.plan.price, group.plan.currency) }}</strong>
          <p><Icon name="bolt" size="sm" aria-hidden="true" />{{ t('packages.instantSummary') }}</p>
        </div>
        <div class="package-detail__checkout">
          <button v-if="group.order_id" class="btn btn-primary package-detail__buy" @click="$emit('checkout')">
            <Icon name="creditCard" size="sm" aria-hidden="true" />{{ t('packages.continuePayment') }}
          </button>
          <template v-else-if="canJoin">
            <label class="package-detail__consent">
              <input :checked="accepted" type="checkbox" @change="$emit('update:accepted', ($event.target as HTMLInputElement).checked)" />
              <span>{{ t('packages.agree') }}</span>
            </label>
            <button class="btn btn-primary package-detail__buy" :disabled="!accepted" @click="$emit('checkout')">
              <Icon name="creditCard" size="sm" aria-hidden="true" />
              {{ t(group.status === 'draft' ? 'packages.buyAndStart' : 'packages.buyAndJoin') }}
            </button>
          </template>
          <p v-else class="package-detail__closed">{{ t(displayStatus === 'settled' ? 'packages.settled' : 'packages.closing') }}</p>
        </div>
      </template>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useNow } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatPaymentAmount } from '@/components/payment/currency'
import { formatDateTimeToMinute } from '@/utils/format'
import type { PackageGroupBuy } from '@/types/packages'

const props = defineProps<{ group: PackageGroupBuy; accepted: boolean }>()
defineEmits<{ checkout: []; 'update:accepted': [value: boolean] }>()
const { t } = useI18n()
const now = useNow({ interval: 1000 })
const tiers = computed(() => props.group.plan.tiers)
const memberCount = computed(() => props.group.final_members ?? props.group.paid_count)
const reachedTier = computed(() => tiers.value.filter(tier => memberCount.value >= tier.members).at(-1))
const reachedQuota = computed(() => props.group.final_quota_usd ?? reachedTier.value?.quota_usd ?? props.group.plan.base_quota_usd)
const periodQuota = computed(() => props.group.plan.base_quota_usd / (props.group.plan.validity_days === 7 ? 1 : 4))
const ended = computed(() => !!props.group.ends_at && Date.parse(props.group.ends_at) <= now.value.getTime())
const displayStatus = computed(() => props.group.status === 'open' && ended.value ? 'closing' : props.group.status)
const canJoin = computed(() => props.group.status !== 'settled' && !props.group.joined && !ended.value)
const countdown = computed(() => {
  const seconds = props.group.status === 'draft'
    ? props.group.plan.group_buy_hours * 3600
    : Math.max(0, Math.floor((Date.parse(props.group.ends_at ?? '') - now.value.getTime()) / 1000)) || 0
  return [Math.floor(seconds / 3600), Math.floor(seconds / 60) % 60, Math.floor(seconds % 60)]
    .map(value => String(value).padStart(2, '0')).join(':')
})
const rules = computed(() => [
  { title: 'packages.instantTitle', text: t('packages.instant'), icon: 'bolt', tone: 'mint' },
  { title: 'packages.quotaSchedule', text: t(props.group.plan.validity_days === 7 ? 'packages.weekSchedule' : 'packages.monthSchedule'), icon: 'calendar', tone: 'blue' },
  { title: 'packages.multiGroupTitle', text: t('packages.multiGroupRule'), icon: 'users', tone: 'blue' },
  { title: 'packages.settlementTitle', text: t('packages.settlementRule'), icon: 'shield', tone: 'amber' },
  { title: 'packages.group', text: props.group.plan.group_name, icon: 'cpu', tone: 'mint' },
] as const)
const quota = (value: number) => `$${value.toLocaleString(undefined, { maximumFractionDigits: 8 })}`
</script>

<style scoped>
.package-detail {
  --detail-ink: #181c1b;
  --detail-muted: #64716b;
  --detail-line: #e4eae6;
  --detail-accent: #16814a;
  min-width: 0;
  color: var(--detail-ink);
  letter-spacing: 0;
}

.package-detail__grid { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(340px, 1fr); gap: 32px; }
.package-detail__product { display: flex; flex-direction: column; min-width: 0; padding: 28px; background: #e4f8ec; }
.package-detail__eyebrow { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; color: var(--detail-accent); font-size: 12px; font-weight: 600; }
.package-detail__pill { border-left: 1px solid #9dcfb0; padding-left: 8px; }
.package-detail__title { margin-top: 14px; font-size: 30px; line-height: 1.3; font-weight: 750; overflow-wrap: anywhere; }
.package-detail__subtitle { max-width: 560px; margin-top: 10px; color: var(--detail-muted); font-size: 13px; line-height: 1.7; overflow-wrap: anywhere; }
.package-detail__visual { display: grid; flex: 1; place-items: center; min-width: 0; padding: 20px 0; }
.package-detail__art { display: block; width: min(100%, 420px); aspect-ratio: 4 / 3; object-fit: contain; }
.package-detail__facts { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; border-top: 1px solid #bfe2cc; padding-top: 22px; }
.package-detail__facts div { min-width: 0; }
.package-detail__facts dt { color: var(--detail-muted); font-size: 12px; }
.package-detail__facts dd { margin-top: 7px; font-size: 24px; line-height: 1.4; font-weight: 700; overflow-wrap: anywhere; }
.package-detail small { font-size: 11px; font-weight: 500; }
.package-detail__benefits { display: flex; flex-wrap: wrap; gap: 12px 20px; margin-top: 22px; color: var(--detail-accent); font-size: 12px; }
.package-detail__benefits span { display: flex; align-items: center; gap: 6px; }
.package-detail__rail { min-width: 0; }
.package-detail h2 { font-size: 15px; line-height: 1.5; font-weight: 700; }
.package-detail__heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; }
.package-detail__status { padding: 4px 8px; border-radius: 4px; background: #edf8f1; color: var(--detail-accent); font-size: 11px; }
.package-detail__progress-label { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 8px; margin-top: 20px; color: var(--detail-muted); font-size: 12px; }
.package-detail__progress-label strong { color: var(--detail-accent); }
.package-detail progress { display: block; width: 100%; height: 5px; margin-top: 10px; border: 0; border-radius: 4px; overflow: hidden; appearance: none; background: #e8efea; }
.package-detail progress::-webkit-progress-bar { background: #e8efea; }
.package-detail progress::-webkit-progress-value { background: #31b86a; }
.package-detail progress::-moz-progress-bar { background: #31b86a; }
.package-detail__tiers { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 100px), 1fr)); gap: 8px; margin-top: 16px; }
.package-detail__tier { display: flex; flex-direction: column; gap: 6px; min-width: 0; padding: 12px 10px; border: 1px solid var(--detail-line); border-radius: 6px; overflow-wrap: anywhere; }
.package-detail__tier-heading { display: flex; align-items: center; justify-content: space-between; gap: 4px; }
.package-detail__tier-heading strong { font-size: 13px; }
.package-detail__tier-heading svg { flex: none; color: var(--detail-accent); }
.package-detail__tier b { margin-top: 4px; font-size: 18px; line-height: 1.4; }
.package-detail__tier > span { color: var(--detail-muted); font-size: 10px; }
.package-detail__tier.is-current { border-color: #42b875; background: #f0faf4; }
.package-detail__quota-summary { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; padding: 16px 0; color: var(--detail-muted); font-size: 12px; }
.package-detail__quota-summary strong { color: var(--detail-accent); font-size: 17px; }
.package-detail__countdown { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; border-top: 1px solid var(--detail-line); padding-top: 14px; font-size: 12px; }
.package-detail__countdown small { display: block; margin-top: 4px; color: var(--detail-muted); }
.package-detail__timer { border-radius: 4px; padding: 6px 10px; background: #222925; color: #fff; font-size: 16px; line-height: 1.3; font-variant-numeric: tabular-nums; white-space: nowrap; }
.package-detail__rules { margin-top: 22px; border-top: 1px solid var(--detail-line); padding-top: 20px; }
.package-detail__rules ul { display: grid; gap: 15px; margin-top: 16px; }
.package-detail__rule { display: flex; gap: 10px; align-items: flex-start; }
.package-detail__rule > div { min-width: 0; }
.package-detail__rule-icon { display: grid; place-items: center; flex: none; width: 30px; height: 30px; border-radius: 6px; }
.package-detail__rule-icon--mint { background: #eaf8ef; color: #16814a; }
.package-detail__rule-icon--blue { background: #eef3ff; color: #4565aa; }
.package-detail__rule-icon--amber { background: #fff5df; color: #9c7216; }
.package-detail__rule h3 { font-size: 12px; font-weight: 650; }
.package-detail__rule p { margin-top: 4px; color: var(--detail-muted); font-size: 12px; line-height: 1.65; overflow-wrap: anywhere; }
.package-detail__action { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 24px; margin-top: 28px; border-top: 1px solid var(--detail-line); padding: 22px 0 4px; }
.package-detail__price { min-width: 0; }
.package-detail__price > span { display: block; color: var(--detail-muted); font-size: 12px; }
.package-detail__price > strong { display: block; margin-top: 2px; color: var(--detail-accent); font-size: 30px; line-height: 1.3; overflow-wrap: anywhere; }
.package-detail__price p { display: flex; align-items: center; gap: 5px; margin-top: 8px; font-size: 12px; color: var(--detail-muted); }
.package-detail__price p svg { flex: none; color: var(--detail-accent); }
.package-detail__checkout { display: flex; flex-direction: column; align-items: flex-end; gap: 12px; min-width: 0; max-width: 100%; }
.package-detail__consent { display: flex; align-items: flex-start; gap: 8px; color: var(--detail-muted); font-size: 12px; line-height: 1.5; cursor: pointer; }
.package-detail__consent input { flex: none; width: 16px; height: 16px; margin-top: 1px; accent-color: #16814a; }
.package-detail__buy { min-height: 44px; min-width: 210px; max-width: 100%; }
.package-detail .btn { gap: 8px; white-space: normal; }
.package-detail .btn svg { flex: none; }
.package-detail__purchased { display: flex; align-items: flex-start; gap: 12px; min-width: 0; }
.package-detail__purchased > svg { flex: none; color: var(--detail-accent); }
.package-detail__purchased strong { font-size: 15px; }
.package-detail__purchased p { margin-top: 6px; color: var(--detail-muted); font-size: 12px; }
.package-detail__links { display: flex; align-items: center; flex-wrap: wrap; gap: 18px; }
.package-detail__order-link { color: var(--detail-muted); font-size: 12px; text-decoration: underline; text-underline-offset: 3px; }
.package-detail__closed { color: var(--detail-muted); font-size: 14px; }

.dark .package-detail { --detail-ink: #edf2ee; --detail-muted: #a5b4ab; --detail-line: #36443b; --detail-accent: #7edda5; }
.dark .package-detail__product { background: #18372a; }
.dark .package-detail__facts, .dark .package-detail__pill { border-color: #3d6550; }
.dark .package-detail__tier.is-current, .dark .package-detail__status { background: #193c2a; }
.dark .package-detail__rule-icon--mint { background: #1a3d2c; color: #7edda5; }
.dark .package-detail__rule-icon--blue { background: #27374c; color: #a4b9ec; }
.dark .package-detail__rule-icon--amber { background: #443b26; color: #eac577; }

@media (max-width: 1100px) {
  .package-detail__grid { grid-template-columns: minmax(0, 1.3fr) minmax(300px, 1fr); gap: 24px; }
  .package-detail__product { padding: 24px; }
  .package-detail__facts dd { font-size: 20px; }
}

@media (max-width: 800px) {
  .package-detail__grid { grid-template-columns: minmax(0, 1fr); }
  .package-detail__art { width: min(100%, 340px); }
  .package-detail__title { font-size: 28px; }
  .package-detail__action { align-items: stretch; }
  .package-detail__checkout { align-items: stretch; flex: 1 1 280px; }
  .package-detail__buy { width: 100%; min-width: 0; }
}

@media (max-width: 420px) {
  .package-detail__product { padding: 20px 16px; }
  .package-detail__title { font-size: 25px; }
  .package-detail__facts { gap: 8px; }
  .package-detail__facts dd { font-size: 18px; }
  .package-detail__facts small { display: block; }
  .package-detail__benefits { gap: 10px 14px; }
  .package-detail__tiers { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .package-detail__tier { padding: 10px 8px; }
  .package-detail__tier b { font-size: 16px; }
  .package-detail__links { width: 100%; justify-content: space-between; }
}
</style>
