<script setup lang="ts">
import { computed } from 'vue'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import TrafficBarChart from '~/components/TrafficBarChart.vue'
import TrafficEntityTable from '~/components/TrafficEntityTable.vue'
import LogStream from '~/components/LogStream.vue'
import { useStats } from '~/composables/stats/useStats'
import { useTrafficStats } from '~/composables/stats/useTrafficStats'
import { useTokens } from '~/composables/tokens/useTokens'
import {
  useTokenTrafficStats,
  useNodeTrafficStats,
  useInboundTrafficStats,
  useDomainTrafficStats,
} from '~/composables/stats/useEntityTraffic'

definePageMeta({
  layout: 'default',
})

useHead({
  title: 'Dashboard',
})

const { data: stats, isLoading, isError, error } = useStats()
const { data: traffic, isLoading: isTrafficLoading } = useTrafficStats()
const { data: tokens, isLoading: isTokensLoading } = useTokens()
const { data: tokenTraffic, isLoading: isTokenTrafficLoading } = useTokenTrafficStats()
const { data: nodeTraffic, isLoading: isNodeTrafficLoading } = useNodeTrafficStats()
const { data: inboundTraffic, isLoading: isInboundTrafficLoading } = useInboundTrafficStats()
const { data: domainTraffic, isLoading: isDomainTrafficLoading } = useDomainTrafficStats()

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

const tokensWithQuota = computed(() => {
  const list = tokens.value ?? []
  return list.filter((t) => t.quota_bytes && t.quota_period)
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
          <div v-if="isTrafficLoading" class="py-4 text-center text-muted-foreground">
            Loading traffic stats...
          </div>
          <div v-else class="space-y-4">
            <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <UiCard v-for="card in trafficCards" :key="card.label" class="p-4">
                <CardContent class="p-0 space-y-2">
                  <p class="text-sm text-muted-foreground">{{ card.label }}</p>
                  <p class="text-3xl font-semibold">{{ card.value }}</p>
                </CardContent>
              </UiCard>
            </div>
            <UiCard class="p-4">
              <CardContent class="p-0">
                <TrafficBarChart
                  :day-upload="traffic?.day_upload_bytes ?? 0"
                  :day-download="traffic?.day_download_bytes ?? 0"
                  :month-upload="traffic?.month_upload_bytes ?? 0"
                  :month-download="traffic?.month_download_bytes ?? 0"
                />
              </CardContent>
            </UiCard>
          </div>
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3">Per-Token Traffic (Today)</h2>
          <UiCard class="p-4">
            <CardContent class="p-0">
              <TrafficEntityTable
                :items="tokenTraffic?.items ?? []"
                :is-loading="isTokenTrafficLoading"
                empty-text="No token traffic recorded yet"
              />
            </CardContent>
          </UiCard>
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3">Per-Node Traffic (Today)</h2>
          <UiCard class="p-4">
            <CardContent class="p-0">
              <TrafficEntityTable
                :items="nodeTraffic?.items ?? []"
                :is-loading="isNodeTrafficLoading"
                empty-text="No node traffic recorded yet"
              />
            </CardContent>
          </UiCard>
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3">Per-Inbound Traffic (Today)</h2>
          <UiCard class="p-4">
            <CardContent class="p-0">
              <TrafficEntityTable
                :items="inboundTraffic?.items ?? []"
                :is-loading="isInboundTrafficLoading"
                empty-text="No inbound traffic recorded yet"
              />
            </CardContent>
          </UiCard>
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3">Per-Domain Traffic (Today)</h2>
          <UiCard class="p-4">
            <CardContent class="p-0">
              <TrafficEntityTable
                :items="domainTraffic?.items ?? []"
                :is-loading="isDomainTrafficLoading"
                empty-text="No domain traffic recorded yet"
              />
            </CardContent>
          </UiCard>
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3">Live Logs</h2>
          <LogStream />
        </div>

        <div>
          <h2 class="text-lg font-semibold mb-3">Tokens with Quota</h2>
          <UiCard class="p-4">
            <CardContent class="p-0">
              <div v-if="isTokensLoading" class="py-4 text-center text-muted-foreground">
                Loading tokens...
              </div>
              <div
                v-else-if="tokensWithQuota.length === 0"
                class="py-4 text-center text-muted-foreground"
              >
                No tokens have quotas configured
              </div>
              <div v-else class="overflow-x-auto rounded-md border">
                <table class="w-full text-sm">
                  <thead class="bg-muted/50">
                    <tr>
                      <th class="px-4 py-2 text-left font-medium">Owner</th>
                      <th class="px-4 py-2 text-left font-medium">Quota</th>
                      <th class="px-4 py-2 text-left font-medium">Period</th>
                      <th class="px-4 py-2 text-left font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="token in tokensWithQuota" :key="token.id" class="border-t">
                      <td class="px-4 py-2">{{ token.owner }}</td>
                      <td class="px-4 py-2">{{ formatBytes(token.quota_bytes ?? 0) }}</td>
                      <td class="px-4 py-2 capitalize">{{ token.quota_period }}</td>
                      <td class="px-4 py-2">
                        <span
                          class="rounded-full px-2 py-0.5 text-xs font-medium uppercase"
                          :class="
                            token.is_active
                              ? 'bg-green-100 text-green-700'
                              : 'bg-red-100 text-red-700'
                          "
                        >
                          {{ token.is_active ? 'Active' : 'Inactive' }}
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </CardContent>
          </UiCard>
        </div>
      </div>
    </ClientOnly>
  </UiPageLayout>
</template>
