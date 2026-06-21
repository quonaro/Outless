<script setup lang="ts">
interface Props {
  title: string
  description?: string
}

defineProps<Props>()
</script>

<template>
  <div class="p-4 sm:p-6">
    <div class="mb-4 flex items-center justify-between sm:mb-6">
      <div>
        <h1 class="text-2xl font-bold sm:text-3xl">{{ title }}</h1>
        <p v-if="description" class="mt-1 text-sm text-muted-foreground sm:text-base">
          {{ description }}
        </p>
      </div>
      <slot name="actions" />
    </div>
    <slot />
  </div>
</template>
