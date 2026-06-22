<script setup lang="ts">
import { computed } from 'vue'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import { useStats } from '~/composables/stats/useStats'
import { useTrafficStats } from '~/composables/stats/useTrafficStats'

definePageMeta({
  layout: 'default',
})

useHead({
  title: 'Dashboard',
})

const { data: stats, isLoading, isError, error } = useStats()
const { data: traffic } = useTrafficStats()

function formatBytes(v: number): string {
  if (v === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.max(0, Math.floor(Math.log10(v) / 3))
  const unit = units[Math.min(i, units.length - 1)]
  const scaled = v / Math.pow(1000, Math.min(i, units.length - 1))
  return `${scaled.toFixed(2)} ${unit}`
}

interface StatCard {
  label: string
  value: string
  hint?: string
}

const cards = computed<StatCard[]>(() => {
  const s = stats.value
  if (!s) return []
  return [
    { label: 'Total nodes', value: String(s.nodes_total) },
    {
      label: 'Active tokens',
      value: String(s.tokens_active),
      hint: `${s.tokens_total} total`,
    },
    { label: 'Groups', value: String(s.groups_total) },
  ]
})

const trafficCards = computed<StatCard[]>(() => {
  const t = traffic.value
  if (!t) return []
  return [
    { label: 'Today upload', value: formatBytes(t.day_upload_bytes) },
    { label: 'Today download', value: formatBytes(t.day_download_bytes) },
    { label: 'Month upload', value: formatBytes(t.month_upload_bytes) },
    { label: 'Month download', value: formatBytes(t.month_download_bytes) },
  ]
})
</script>

<template>
  <UiPageLayout title="Dashboard" description="Overview of Outless state">
    <ClientOnly>
      <template #fallback>
        <div class="py-8 text-center text-muted-foreground">Loading stats...</div>
      </template>

      <div v-if="isLoading" class="py-8 text-center text-muted-foreground">Loading stats...</div>
      <div v-else-if="isError" class="py-8 text-center text-destructive">
        Failed to load stats: {{ error?.message }}
      </div>
      <div v-else class="space-y-6">
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <UiCard v-for="card in cards" :key="card.label" class="p-4">
            <CardContent class="p-0 space-y-2">
              <p class="text-sm text-muted-foreground">{{ card.label }}</p>
              <p class="text-3xl font-semibold">{{ card.value }}</p>
              <p v-if="card.hint" class="text-xs text-muted-foreground">
                {{ card.hint }}
              </p>
            </CardContent>
          </UiCard>
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3">Traffic</h2>
          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <UiCard v-for="card in trafficCards" :key="card.label" class="p-4">
              <CardContent class="p-0 space-y-2">
                <p class="text-sm text-muted-foreground">{{ card.label }}</p>
                <p class="text-3xl font-semibold">{{ card.value }}</p>
              </CardContent>
            </UiCard>
          </div>
        </div>
      </div>
    </ClientOnly>
  </UiPageLayout>
</template>
