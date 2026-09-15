<template>
  <BaseDialog :show="show" :title="t('payment.paymentMethod')" :close-on-escape="!submitting" :close-on-click-outside="!submitting" :show-close-button="!submitting" @close="close">
    <PaymentCheckout
      embedded
      :initial-group="group"
      :initial-package="plan"
      :initial-payment="initialPayment"
      :initial-terms-accepted="termsAccepted"
      @submitting="submitting = $event"
      @close="close"
      @success="$emit('success')"
      @pending="$emit('pending', $event)"
      @settled="$emit('settled')"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PaymentCheckout from '@/components/payment/PaymentCheckout.vue'
import type { PackageGroupBuy, PackagePlan } from '@/types/packages'
import type { PaymentRecoverySnapshot } from '@/components/payment/paymentFlow'
defineProps<{ show: boolean; group?: PackageGroupBuy; plan?: PackagePlan; initialPayment?: PaymentRecoverySnapshot; termsAccepted?: boolean }>()
const emit = defineEmits<{ close: []; success: []; pending: [payment: PaymentRecoverySnapshot]; settled: [] }>()
const { t } = useI18n()
const submitting = ref(false)
function close() { if (!submitting.value) emit('close') }
</script>
