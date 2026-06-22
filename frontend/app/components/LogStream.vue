<script setup lang="ts">
defineProps<{
  lines: string[]
}>()

function levelClass(line: string): string {
  const m = line.match(/^\[(\w+)\]/)
  if (!m || !m[1]) return ''
  const lvl = m[1].toUpperCase()
  if (lvl.startsWith('DEBU')) return 'text-blue-400'
  if (lvl.startsWith('INFO')) return 'text-green-400'
  if (lvl.startsWith('WARN')) return 'text-amber-400'
  if (lvl.startsWith('ERRO') || lvl.startsWith('FATA') || lvl.startsWith('PANI'))
    return 'text-red-400'
  return ''
}
</script>

<template>
  <div
    class="rounded-md border bg-black font-mono text-xs p-3 overflow-hidden h-full flex flex-col"
  >
    <div class="flex-1 overflow-y-auto space-y-1 min-h-0">
      <div v-if="lines.length === 0" class="text-muted-foreground opacity-50">
        Waiting for logs...
      </div>
      <div v-for="(line, i) in lines" :key="i" class="flex gap-3 break-all">
        <span class="select-none text-right text-muted-foreground/50 w-8 flex-shrink-0">
          {{ i + 1 }}
        </span>
        <span :class="levelClass(line)">{{ line }}</span>
      </div>
    </div>
  </div>
</template>
