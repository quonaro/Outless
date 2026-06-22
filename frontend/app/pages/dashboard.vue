<script setup lang="ts">
import { computed } from 'vue'
import {
  Server,
  KeyRound,
  Users,
  ArrowUpFromLine,
  ArrowDownToLine,
  TrendingUp,
  TrendingDown,
  BarChart3,
  Key,
  ArrowLeftRight,
  Globe,
  Terminal,
  Gauge,
  ShieldCheck,
  ShieldX,
  LayoutGrid,
  Copy,
  Trash2,
} from 'lucide-vue-next'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import TrafficBarChart from '~/components/TrafficBarChart.vue'
import TrafficEntityTable from '~/components/TrafficEntityTable.vue'
import LogStream from '~/components/LogStream.vue'
import { useStats } from '~/composables/stats/useStats'
import { useTrafficStats } from '~/composables/stats/useTrafficStats'
import { useTokens } from '~/composables/tokens/useTokens'
import { useLogStream } from '~/composables/stats/useLogStream'
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
const { lines: logLines, isConnected: isLogConnected } = useLogStream()
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

function copyLogs() {
  navigator.clipboard.writeText(logLines.value.join('\n'))
}

function clearLogs() {
  logLines.value = []
}

interface StatCard {
  label: string
  value: string
  hint?: string
  icon: unknown
  iconColor: string
  iconBg: string
}

const cards = computed<StatCard[]>(() => {
  const s = stats.value
  if (!s) return []
  return [
    {
      label: 'Total nodes',
      value: String(s.nodes_total),
      icon: Server,
      iconColor: 'text-sky-600',
      iconBg: 'bg-sky-500/10',
    },
    {
      label: 'Active tokens',
      value: String(s.tokens_active),
      icon: KeyRound,
      iconColor: 'text-amber-600',
      iconBg: 'bg-amber-500/10',
    },
    {
      label: 'Total tokens',
      value: String(s.tokens_total),
      icon: KeyRound,
      iconColor: 'text-orange-600',
      iconBg: 'bg-orange-500/10',
    },
    {
      label: 'Groups',
      value: String(s.groups_total),
      icon: Users,
      iconColor: 'text-violet-600',
      iconBg: 'bg-violet-500/10',
    },
  ]
})

const trafficCards = computed<StatCard[]>(() => {
  const t = traffic.value
  if (!t) return []
  return [
    {
      label: 'Today upload',
      value: formatBytes(t.day_upload_bytes),
      icon: ArrowUpFromLine,
      iconColor: 'text-blue-600',
      iconBg: 'bg-blue-500/10',
    },
    {
      label: 'Today download',
      value: formatBytes(t.day_download_bytes),
      icon: ArrowDownToLine,
      iconColor: 'text-emerald-600',
      iconBg: 'bg-emerald-500/10',
    },
    {
      label: 'Month upload',
      value: formatBytes(t.month_upload_bytes),
      icon: TrendingUp,
      iconColor: 'text-indigo-600',
      iconBg: 'bg-indigo-500/10',
    },
    {
      label: 'Month download',
      value: formatBytes(t.month_download_bytes),
      icon: TrendingDown,
      iconColor: 'text-rose-600',
      iconBg: 'bg-rose-500/10',
    },
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
      <div v-else class="space-y-8">
        <div class="grid grid-cols-1 xl:grid-cols-2 gap-8">
          <div class="space-y-8 flex flex-col h-full">
            <div>
              <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
                <LayoutGrid class="h-5 w-5 text-primary" />
                Overview
              </h2>
              <div class="grid gap-4 md:grid-cols-2">
                <UiCard v-for="card in cards" :key="card.label" class="p-4">
                  <CardContent class="p-0">
                    <div class="flex items-center gap-3">
                      <div class="rounded-xl p-2.5" :class="card.iconBg">
                        <component :is="card.icon" :class="['h-5 w-5', card.iconColor]" />
                      </div>
                      <div class="min-w-0">
                        <p class="text-sm text-muted-foreground">{{ card.label }}</p>
                        <p class="text-2xl font-bold">{{ card.value }}</p>
                        <p v-if="card.hint" class="text-xs text-muted-foreground">
                          {{ card.hint }}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </UiCard>
              </div>
            </div>
            <div class="flex-1 flex flex-col min-h-0">
              <div class="flex items-center justify-between mb-3">
                <h2 class="text-lg font-semibold flex items-center gap-2">
                  <Terminal class="h-5 w-5 text-green-500" />
                  Live Logs
                  <span
                    class="inline-block w-2 h-2 rounded-full ml-1"
                    :class="isLogConnected ? 'bg-green-500' : 'bg-red-500'"
                  />
                </h2>
                <div class="flex items-center gap-1">
                  <button
                    class="inline-flex items-center justify-center rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
                    title="Copy logs"
                    @click="copyLogs"
                  >
                    <Copy class="h-4 w-4" />
                  </button>
                  <button
                    class="inline-flex items-center justify-center rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
                    title="Clear logs"
                    @click="clearLogs"
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </div>
              <LogStream :lines="logLines" class="flex-1 min-h-0" />
            </div>
          </div>

          <div class="flex flex-col h-full">
            <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
              <BarChart3 class="h-5 w-5 text-sky-500" />
              Traffic
            </h2>
            <div v-if="isTrafficLoading" class="py-4 text-center text-muted-foreground">
              Loading traffic stats...
            </div>
            <div v-else class="space-y-8 flex-1 flex flex-col min-h-0">
              <div class="grid gap-4 md:grid-cols-2">
                <UiCard v-for="card in trafficCards" :key="card.label" class="p-4">
                  <CardContent class="p-0">
                    <div class="flex items-center gap-3">
                      <div class="rounded-xl p-2.5" :class="card.iconBg">
                        <component :is="card.icon" :class="['h-5 w-5', card.iconColor]" />
                      </div>
                      <div class="min-w-0">
                        <p class="text-sm text-muted-foreground">{{ card.label }}</p>
                        <p class="text-2xl font-bold">{{ card.value }}</p>
                      </div>
                    </div>
                  </CardContent>
                </UiCard>
              </div>
              <div class="flex-1 flex flex-col min-h-0">
                <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
                  <BarChart3 class="h-5 w-5 text-sky-500" />
                  Traffic Chart
                </h2>
                <UiCard class="p-4 flex-1 flex flex-col min-h-0">
                  <CardContent class="p-0 flex-1 flex flex-col min-h-0">
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
          </div>
        </div>

        <div class="grid grid-cols-1 xl:grid-cols-2 gap-8 items-start">
          <div class="space-y-8">
            <div>
              <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
                <Key class="h-5 w-5 text-amber-500" />
                Per-Token Traffic (Today)
              </h2>
              <UiCard class="p-4">
                <CardContent class="p-0">
                  <TrafficEntityTable
                    :items="tokenTraffic?.items ?? []"
                    :is-loading="isTokenTrafficLoading"
                    empty-text="No token traffic recorded yet"
                    :name-icon="Key"
                    :row-icon="Key"
                    icon-color="text-amber-500"
                  />
                </CardContent>
              </UiCard>
            </div>

            <div>
              <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
                <ArrowLeftRight class="h-5 w-5 text-violet-500" />
                Per-Inbound Traffic (Today)
              </h2>
              <UiCard class="p-4">
                <CardContent class="p-0">
                  <TrafficEntityTable
                    :items="inboundTraffic?.items ?? []"
                    :is-loading="isInboundTrafficLoading"
                    empty-text="No inbound traffic recorded yet"
                    :name-icon="ArrowLeftRight"
                    :row-icon="ArrowLeftRight"
                    icon-color="text-violet-500"
                  />
                </CardContent>
              </UiCard>
            </div>

            <div>
              <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
                <Gauge class="h-5 w-5 text-rose-500" />
                Tokens with Quota
              </h2>
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
                          <td class="px-4 py-2">
                            <div class="flex items-center gap-2">
                              <KeyRound class="h-4 w-4 text-muted-foreground" />
                              {{ token.owner }}
                            </div>
                          </td>
                          <td class="px-4 py-2">{{ formatBytes(token.quota_bytes ?? 0) }}</td>
                          <td class="px-4 py-2 capitalize">{{ token.quota_period }}</td>
                          <td class="px-4 py-2">
                            <div class="flex items-center gap-1.5">
                              <component
                                :is="token.is_active ? ShieldCheck : ShieldX"
                                :class="
                                  token.is_active
                                    ? 'h-4 w-4 text-green-600'
                                    : 'h-4 w-4 text-red-600'
                                "
                              />
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
                            </div>
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </CardContent>
              </UiCard>
            </div>
          </div>

          <div class="space-y-8">
            <div>
              <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
                <Server class="h-5 w-5 text-sky-500" />
                Per-Node Traffic (Today)
              </h2>
              <UiCard class="p-4">
                <CardContent class="p-0">
                  <TrafficEntityTable
                    :items="nodeTraffic?.items ?? []"
                    :is-loading="isNodeTrafficLoading"
                    empty-text="No node traffic recorded yet"
                    :name-icon="Server"
                    :row-icon="Server"
                    icon-color="text-sky-500"
                  />
                </CardContent>
              </UiCard>
            </div>

            <div>
              <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
                <Globe class="h-5 w-5 text-emerald-500" />
                Per-Domain Traffic (Today)
              </h2>
              <UiCard class="p-4">
                <CardContent class="p-0">
                  <TrafficEntityTable
                    :items="domainTraffic?.items ?? []"
                    :is-loading="isDomainTrafficLoading"
                    empty-text="No domain traffic recorded yet"
                    :name-icon="Globe"
                    :row-icon="Globe"
                    icon-color="text-emerald-500"
                  />
                </CardContent>
              </UiCard>
            </div>
          </div>
        </div>
      </div>
    </ClientOnly>
  </UiPageLayout>
</template>
