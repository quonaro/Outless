<script setup lang="ts">
import { computed } from 'vue'
import type { TrafficEntityItem } from '~/utils/schemas/stats'

const props = defineProps<{
  items: TrafficEntityItem[]
  isLoading: boolean
  emptyText?: string
  nameIcon?: unknown
  rowIcon?: unknown
  iconColor?: string
}>()

function formatBytes(v: number): string {
  if (v === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.max(0, Math.floor(Math.log10(v) / 3))
  const unit = units[Math.min(i, units.length - 1)]
  const scaled = v / Math.pow(1000, Math.min(i, units.length - 1))
  return `${scaled.toFixed(2)} ${unit}`
}

const processedItems = computed(() => {
  const map = new Map<string, TrafficEntityItem>()
  for (const item of props.items) {
    const key = item.name || item.id
    const existing = map.get(key)
    if (existing) {
      existing.upload_bytes += item.upload_bytes
      existing.download_bytes += item.download_bytes
      existing.total_bytes += item.total_bytes
    } else {
      map.set(key, { ...item, id: key })
    }
  }
  return Array.from(map.values()).sort((a, b) => b.total_bytes - a.total_bytes)
})
</script>

<template>
  <div>
    <div v-if="isLoading" class="py-4 text-center text-muted-foreground">Loading...</div>
    <div v-else-if="items.length === 0" class="py-4 text-center text-muted-foreground">
      {{ emptyText ?? 'No data' }}
    </div>
    <div v-else class="overflow-x-auto rounded-md border">
      <table class="w-full text-sm">
        <thead class="bg-muted/50">
          <tr>
            <th class="px-4 py-2 text-left font-medium">
              <div class="flex items-center gap-1.5">
                <component
                  :is="nameIcon"
                  v-if="nameIcon"
                  :class="['h-4 w-4', iconColor ?? 'text-muted-foreground']"
                />
                Name
              </div>
            </th>
            <th class="px-4 py-2 text-right font-medium">Upload</th>
            <th class="px-4 py-2 text-right font-medium">Download</th>
            <th class="px-4 py-2 text-right font-medium">Total</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in processedItems" :key="item.id" class="border-t">
            <td class="px-4 py-2 truncate max-w-xs">
              <div class="flex items-center gap-2">
                <component
                  :is="rowIcon"
                  v-if="rowIcon"
                  :class="['h-4 w-4 flex-shrink-0', iconColor ?? 'text-muted-foreground']"
                />
                {{ item.name || item.id }}
              </div>
            </td>
            <td class="px-4 py-2 text-right">{{ formatBytes(item.upload_bytes) }}</td>
            <td class="px-4 py-2 text-right">{{ formatBytes(item.download_bytes) }}</td>
            <td class="px-4 py-2 text-right font-medium">{{ formatBytes(item.total_bytes) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
