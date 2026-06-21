<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "~/utils"

const props = defineProps<{
  class?: HTMLAttributes["class"]
}>()
</script>

<template>
  <div
    :class="
      cn(
        'rounded-xl border bg-card text-card-foreground shadow [scrollbar-width:thin] [scrollbar-color:rgba(148,163,184,0.45)_transparent] [&::-webkit-scrollbar]:h-2 [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-zinc-600/55 hover:[&::-webkit-scrollbar-thumb]:bg-zinc-500/75',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
