<script setup lang="ts">
import { ref, computed } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { Activity, ArrowUp, ArrowDown, Wifi, Server, Fingerprint } from 'lucide-vue-next'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'

interface Connection {
  id: string
  user: string
  node_id: string
  inbound: string
  domain: string
  upload: number
  download: number
}

interface ConnectionsResponse {
  upload_total: number
  download_total: number
  connections: Connection[]
}

const config = useRuntimeConfig()

const data = ref<ConnectionsResponse | null>(null)
const isLoading = ref(true)

async function load() {
  try {
    const res = await $fetch<ConnectionsResponse>(`${config.public.apiBase}/v1/connections`)
    data.value = res
  } catch {
    // silently ignore on auto-refresh
  } finally {
    isLoading.value = false
  }
}

load()
useIntervalFn(load, 5000)

const connections = computed<Connection[]>(() => data.value?.connections ?? [])
const uploadTotal = computed(() => data.value?.upload_total ?? 0)
const downloadTotal = computed(() => data.value?.download_total ?? 0)

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / k ** i).toFixed(2))} ${sizes[i]}`
}

function parseTokenID(user: string): string {
  const parts = user.split('-')
  if (parts.length >= 4 && parts[0] === 't' && parts[2] === 'n') {
    return parts[1] ?? user
  }
  return user
}
</script>

<template>
  <UiPageLayout title="Connections" description="Active sing-box connections in real time">
    <div class="space-y-4">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <UiCard class="p-4">
          <CardContent class="p-0 flex items-center gap-3">
            <Wifi class="h-5 w-5 text-orange-500" />
            <div>
              <p class="text-xs text-muted-foreground">Active connections</p>
              <p class="text-lg font-semibold">{{ connections.length }}</p>
            </div>
          </CardContent>
        </UiCard>
        <UiCard class="p-4">
          <CardContent class="p-0 flex items-center gap-3">
            <ArrowUp class="h-5 w-5 text-emerald-500" />
            <div>
              <p class="text-xs text-muted-foreground">Upload total</p>
              <p class="text-lg font-semibold">{{ formatBytes(uploadTotal) }}</p>
            </div>
          </CardContent>
        </UiCard>
        <UiCard class="p-4">
          <CardContent class="p-0 flex items-center gap-3">
            <ArrowDown class="h-5 w-5 text-blue-500" />
            <div>
              <p class="text-xs text-muted-foreground">Download total</p>
              <p class="text-lg font-semibold">{{ formatBytes(downloadTotal) }}</p>
            </div>
          </CardContent>
        </UiCard>
      </div>

      <div v-if="isLoading && !data" class="py-8 text-center text-muted-foreground">
        Loading connections...
      </div>

      <div v-else-if="connections.length === 0" class="py-8 text-center text-muted-foreground">
        No active connections
      </div>

      <div v-else class="space-y-2">
        <UiCard v-for="conn in connections" :key="conn.id" class="p-3">
          <CardContent class="p-0">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-center gap-2 min-w-0">
                <Activity class="h-4 w-4 shrink-0 text-orange-500" />
                <span class="text-sm font-medium truncate">{{ parseTokenID(conn.user) }}</span>
              </div>
              <div class="flex items-center gap-3 text-xs text-muted-foreground">
                <span v-if="conn.domain" class="flex items-center gap-1">
                  <Server class="h-3 w-3" />
                  {{ conn.domain }}
                </span>
                <span v-if="conn.inbound" class="flex items-center gap-1">
                  <Fingerprint class="h-3 w-3" />
                  {{ conn.inbound }}
                </span>
              </div>
              <div class="flex items-center gap-3 text-xs">
                <span class="flex items-center gap-1 text-emerald-600">
                  <ArrowUp class="h-3 w-3" />
                  {{ formatBytes(conn.upload) }}
                </span>
                <span class="flex items-center gap-1 text-blue-600">
                  <ArrowDown class="h-3 w-3" />
                  {{ formatBytes(conn.download) }}
                </span>
              </div>
            </div>
          </CardContent>
        </UiCard>
      </div>
    </div>
  </UiPageLayout>
</template>
