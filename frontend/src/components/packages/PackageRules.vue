<template>
  <section class="rounded-xl border border-amber-200 bg-amber-50/60 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-900/10 dark:text-amber-200">
    <h3 class="mb-2 font-semibold">{{ t(standalone ? 'packages.singleRules' : 'packages.rules') }}</h3>
    <ul class="list-disc space-y-1 pl-5">
      <li>{{ t('packages.instant') }}</li>
      <li>{{ t(standalone ? validityDays === 30 ? 'packages.monthSchedule' : 'packages.weekSchedule' : 'packages.settlement') }}</li>
      <li>{{ t(standalone ? 'packages.singleTerms' : 'packages.noRefund') }}</li>
    </ul>
  </section>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ standalone?: boolean; validityDays?: number }>()
const { t } = useI18n()
</script>
