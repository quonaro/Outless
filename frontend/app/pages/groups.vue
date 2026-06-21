<script setup lang="ts">
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiGroupList from '~/components/ui/group-list/group-list.vue'

definePageMeta({
  layout: 'default',
})
</script>

<template>
  <UiPageLayout
    title="Groups"
    description="Organize your nodes into groups"
  >
    <UiGroupList />
  </UiPageLayout>
</template>
