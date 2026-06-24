<script setup lang="ts">
import { computed } from 'vue'
import type { TrafficEntityItem } from '~/utils/schemas/stats'

const props = defineProps<{
  items: TrafficEntityItem[]
  isLoading: boolean
  emptyText?: string
}>()

function formatBytes(v: number): string {
  if (v === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.max(0, Math.floor(Math.log10(v) / 3))
  const unit = units[Math.min(i, units.length - 1)]
  const scaled = v / Math.pow(1000, Math.min(i, units.length - 1))
  return `${scaled.toFixed(2)} ${unit}`
}

function extractNodeInfo(url: string): { name: string; host?: string } {
  if (!url.startsWith('vless://')) return { name: url }

  const hashIdx = url.indexOf('#')
  let remark: string | undefined
  if (hashIdx !== -1) {
    remark = decodeURIComponent(url.slice(hashIdx + 1))
  }

  const atIdx = url.indexOf('@')
  let host: string | undefined
  if (atIdx !== -1) {
    const endIdx = url.indexOf('?', atIdx)
    const hostPort = endIdx !== -1 ? url.slice(atIdx + 1, endIdx) : url.slice(atIdx + 1)
    const hashIdx2 = hostPort.indexOf('#')
    host = hashIdx2 !== -1 ? hostPort.slice(0, hashIdx2) : hostPort
  }

  return { name: remark || url, host }
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
    <div v-else class="overflow-auto max-h-[14rem]">
      <table class="w-full text-sm">
        <thead class="bg-muted/50 sticky top-0 z-10">
          <tr>
            <th class="px-4 py-2 text-left font-medium">Name</th>
            <th class="px-4 py-2 text-right font-medium whitespace-nowrap">Upload</th>
            <th class="px-4 py-2 text-right font-medium whitespace-nowrap">Download</th>
            <th class="px-4 py-2 text-right font-medium whitespace-nowrap">Total</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in processedItems" :key="item.id" class="border-t">
            <td class="px-4 py-2">
              <div class="min-w-0">
                <template v-if="item.name?.startsWith('vless://')">
                  <div class="truncate font-medium">
                    {{ extractNodeInfo(item.name).name }}
                  </div>
                  <div class="text-xs text-muted-foreground truncate">
                    {{ extractNodeInfo(item.name).host }}
                  </div>
                </template>
                <template v-else>
                  <div class="truncate">{{ item.name || item.id }}</div>
                </template>
              </div>
            </td>
            <td class="px-4 py-2 text-right whitespace-nowrap">
              {{ formatBytes(item.upload_bytes) }}
            </td>
            <td class="px-4 py-2 text-right whitespace-nowrap">
              {{ formatBytes(item.download_bytes) }}
            </td>
            <td class="px-4 py-2 text-right font-medium whitespace-nowrap">
              {{ formatBytes(item.total_bytes) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
