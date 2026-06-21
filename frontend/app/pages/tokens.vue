<script setup lang="ts">
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import TokenList from '~/components/TokenList.vue'

definePageMeta({
  layout: 'default',
})
</script>

<template>
  <UiPageLayout
    title="Tokens"
    description="Manage your access tokens"
  >
    <ClientOnly>
      <template #fallback>
        <div class="py-8 text-center text-muted-foreground">
          Loading tokens...
        </div>
      </template>
      <TokenList />
    </ClientOnly>
  </UiPageLayout>
</template>
