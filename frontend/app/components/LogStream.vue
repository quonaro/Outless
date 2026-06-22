<script setup lang="ts">
import { useLogStream } from '~/composables/stats/useLogStream'

const { lines, isConnected } = useLogStream()
</script>

<template>
  <div class="rounded-md border bg-black text-green-400 font-mono text-xs p-3 overflow-hidden">
    <div class="flex items-center justify-between mb-2">
      <span class="font-semibold text-green-300">Live Logs</span>
      <span
        class="inline-block w-2 h-2 rounded-full"
        :class="isConnected ? 'bg-green-500' : 'bg-red-500'"
      />
    </div>
    <div class="h-48 overflow-y-auto space-y-1">
      <div v-if="lines.length === 0" class="text-muted-foreground opacity-50">
        Waiting for logs...
      </div>
      <div v-for="(line, i) in lines" :key="i" class="break-all">
        {{ line }}
      </div>
    </div>
  </div>
</template>
