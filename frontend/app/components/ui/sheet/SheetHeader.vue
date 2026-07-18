<script setup lang="ts">
import { cn } from '~/utils'
import type { HTMLAttributes } from 'vue'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div :class="cn('flex flex-col shrink-0 gap-y-2 text-center sm:text-left', props.class)">
    <slot />
  </div>
</template>
