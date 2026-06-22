<script setup lang="ts">
defineProps<{
  lines: string[]
}>()
</script>

<template>
  <div
    class="rounded-md border bg-black text-green-400 font-mono text-xs p-3 overflow-hidden h-full flex flex-col"
  >
    <div class="flex-1 overflow-y-auto space-y-1 min-h-0">
      <div v-if="lines.length === 0" class="text-muted-foreground opacity-50">
        Waiting for logs...
      </div>
      <div v-for="(line, i) in lines" :key="i" class="break-all">
        {{ line }}
      </div>
    </div>
  </div>
</template>
