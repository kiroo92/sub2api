<template>
  <AppLayout>
    <PaymentCheckout>
      <template #shop>
        <PackageShop @select="openIndividualPayment" @start="openGroupDetail" />
      </template>
    </PaymentCheckout>
    <PackagePaymentDialog
      v-if="paymentPlan && showPayment"
      :show="showPayment"
      :plan="paymentPlan"
      :initial-payment="pendingPayments.get(paymentPlan.id)"
      @pending="pendingPayments.set(paymentPlan.id, $event)"
      @settled="pendingPayments.delete(paymentPlan.id)"
      @close="showPayment = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PaymentCheckout from '@/components/payment/PaymentCheckout.vue'
import PackageShop from '@/components/packages/PackageShop.vue'
import PackagePaymentDialog from '@/components/packages/PackagePaymentDialog.vue'
import type { PaymentRecoverySnapshot } from '@/components/payment/paymentFlow'
import type { PackageGroupBuy, PackagePlan } from '@/types/packages'

const router = useRouter()
const paymentPlan = ref<PackagePlan | null>(null)
const showPayment = ref(false)
// Only orders admitted from this shop session may be restored into a single buy.
const pendingPayments = new Map<number, PaymentRecoverySnapshot>()
function openIndividualPayment(plan: PackagePlan) {
  if (showPayment.value) return
  paymentPlan.value = plan
  showPayment.value = true
}
function openGroupDetail(_plan: PackagePlan, group: PackageGroupBuy) {
  void router.push(`/package-groups/${group.id}`)
}
</script>
