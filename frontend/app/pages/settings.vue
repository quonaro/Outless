<script setup lang="ts">
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiSettingsPanel from '~/components/ui/settings-panel/settings-panel.vue'

definePageMeta({
  layout: 'default',
})

useHead({
  title: 'Settings',
})
</script>

<template>
  <UiPageLayout title="Settings" description="Configure server and routing options">
    <ClientOnly>
      <template #fallback>
        <div class="py-8 text-center text-muted-foreground">Loading settings...</div>
      </template>
      <UiSettingsPanel />
    </ClientOnly>
  </UiPageLayout>
</template>
