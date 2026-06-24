<script setup lang="ts">
import { computed } from 'vue'
import { Activity, ArrowUp, ArrowDown, Wifi, History, Search } from 'lucide-vue-next'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import { useConnectionsStream } from '~/composables/stats/useConnectionsStream'
import { useConnectionsSearch } from '~/composables/useConnectionsSearch'
import { formatBytes } from '~/utils'

const { data } = useConnectionsStream()
const searchQuery = useConnectionsSearch()

const uploadTotal = computed(() => data.value?.upload_total ?? 0)
const downloadTotal = computed(() => data.value?.download_total ?? 0)
const totalFlows = computed(() => data.value?.connections.length ?? 0)
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
              <p class="text-lg font-semibold">{{ totalFlows }}</p>
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

      <div class="flex gap-2">
        <NuxtLink
          to="/connections/active"
          class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
          :class="
            $route.path === '/connections/active'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80'
          "
        >
          <Activity class="h-3.5 w-3.5" />
          Active
        </NuxtLink>
        <NuxtLink
          to="/connections/history"
          class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
          :class="
            $route.path === '/connections/history'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80'
          "
        >
          <History class="h-3.5 w-3.5" />
          History (30d)
        </NuxtLink>
      </div>

      <div class="relative">
        <Search class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by user, node or domain..."
          class="w-full rounded-md border border-input bg-background py-2 pl-9 pr-3 text-sm text-foreground ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </div>

      <NuxtPage />
    </div>
  </UiPageLayout>
</template>
