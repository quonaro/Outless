<script setup lang="ts">
interface Props {
  title: string
  description?: string
}

defineProps<Props>()
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-3xl font-bold">{{ title }}</h1>
        <p v-if="description" class="text-muted-foreground mt-1">{{ description }}</p>
      </div>
      <slot name="actions" />
    </div>
    <slot />
  </div>
</template>
