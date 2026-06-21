<script setup lang="ts">
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiInboundList from '~/components/ui/inbound-list/inbound-list.vue'

definePageMeta({
  layout: 'default',
})

useHead({
  title: 'Inbounds',
})
</script>

<template>
  <UiPageLayout title="Inbounds" description="Manage VLESS REALITY entry points">
    <UiInboundList />
  </UiPageLayout>
</template>
