<script setup lang="ts">
import { computed, ref } from 'vue'
import { Activity, ArrowUp, ArrowDown, Globe, ChevronDown, ChevronUp } from 'lucide-vue-next'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import { useConnectionsStream } from '~/composables/stats/useConnectionsStream'
import { useTokens } from '~/composables/tokens/useTokens'
import { useNodes } from '~/composables/nodes/useNodes'
import { useConnectionsSearch } from '~/composables/useConnectionsSearch'
import { formatBytes } from '~/utils'

const { data, isConnected } = useConnectionsStream()
const { data: tokens } = useTokens()
const { data: nodes } = useNodes()
const searchQuery = useConnectionsSearch()

interface FlowGroup {
  key: string
  user: string
  node_id: string
  owner: string
  nodeCountry: string
  count: number
  upload: number
  download: number
  flows: {
    id: string
    domain: string
    inbound: string
    source_ip: string
    upload: number
    download: number
  }[]
}

const groups = computed<FlowGroup[]>(() => {
  const raw = data.value?.connections ?? []
  const map = new Map<string, FlowGroup>()
  for (const conn of raw) {
    const key = `${conn.user}||${conn.node_id}`
    const existing = map.get(key)
    const flow = {
      id: conn.id,
      domain: conn.domain,
      inbound: conn.inbound,
      source_ip: conn.source_ip,
      upload: conn.upload,
      download: conn.download,
    }
    if (existing) {
      existing.count++
      existing.upload += conn.upload
      existing.download += conn.download
      existing.flows.push(flow)
    } else {
      map.set(key, {
        key,
        user: conn.user,
        node_id: conn.node_id,
        owner: resolveOwner(conn.user),
        nodeCountry: resolveNode(conn.node_id),
        count: 1,
        upload: conn.upload,
        download: conn.download,
        flows: [flow],
      })
    }
  }
  return Array.from(map.values())
})

const expandedKeys = ref<Set<string>>(new Set())

function toggleGroup(key: string) {
  const set = new Set(expandedKeys.value)
  if (set.has(key)) {
    set.delete(key)
  } else {
    set.add(key)
  }
  expandedKeys.value = set
}

const tokenMap = computed(() => {
  const map = new Map<string, string>()
  if (tokens.value) {
    for (const t of tokens.value) {
      map.set(t.id, t.owner)
    }
  }
  return map
})

const nodeMap = computed(() => {
  const map = new Map<string, string>()
  if (nodes.value) {
    for (const n of nodes.value) {
      map.set(n.id, n.country)
    }
  }
  return map
})

function parseTokenID(user: string): string {
  const parts = user.split('-')
  if (parts.length >= 4 && parts[0] === 't' && parts[2] === 'n') {
    return parts[1] ?? user
  }
  return user
}

function resolveOwner(user: string): string {
  const tokenID = parseTokenID(user)
  return tokenMap.value.get(tokenID) ?? tokenID
}

function resolveNode(nodeID: string): string {
  if (!nodeID) return ''
  return nodeMap.value.get(nodeID) ?? nodeID
}

const filteredGroups = computed<FlowGroup[]>(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return groups.value
  return groups.value
    .map((g) => {
      const matchMeta =
        g.owner.toLowerCase().includes(q) ||
        g.nodeCountry.toLowerCase().includes(q) ||
        g.node_id.toLowerCase().includes(q)
      if (matchMeta) return g
      const domainFlows = g.flows.filter(
        (f) =>
          f.domain.toLowerCase().includes(q) ||
          f.inbound.toLowerCase().includes(q) ||
          f.source_ip.toLowerCase().includes(q)
      )
      if (domainFlows.length > 0) {
        return { ...g, flows: domainFlows, count: domainFlows.length }
      }
      return null
    })
    .filter((g): g is FlowGroup => g !== null)
})
</script>

<template>
  <div>
    <div v-if="!isConnected && !data" class="py-8 text-center text-muted-foreground">
      Loading connections...
    </div>

    <div v-else-if="filteredGroups.length === 0" class="py-8 text-center text-muted-foreground">
      No active connections
    </div>

    <div v-else class="space-y-2">
      <UiCard
        v-for="g in filteredGroups"
        :key="g.key"
        class="p-3 cursor-pointer"
        @click="toggleGroup(g.key)"
      >
        <CardContent class="p-0">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex items-center gap-2 min-w-0">
              <Activity class="h-4 w-4 shrink-0 text-orange-500" />
              <span class="text-sm font-medium truncate">{{ g.owner }}</span>
              <span
                class="inline-flex items-center rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground"
              >
                {{ g.count }}
              </span>
              <component
                :is="expandedKeys.has(g.key) ? ChevronUp : ChevronDown"
                class="h-3 w-3 text-muted-foreground"
              />
            </div>
            <div class="flex items-center gap-3 text-xs text-muted-foreground">
              <span v-if="g.nodeCountry" class="flex items-center gap-1">
                <Globe class="h-3 w-3" />
                {{ g.nodeCountry }}
              </span>
            </div>
            <div class="flex items-center gap-3 text-xs">
              <span class="flex items-center gap-1 text-emerald-600">
                <ArrowUp class="h-3 w-3" />
                {{ formatBytes(g.upload) }}
              </span>
              <span class="flex items-center gap-1 text-blue-600">
                <ArrowDown class="h-3 w-3" />
                {{ formatBytes(g.download) }}
              </span>
            </div>
          </div>

          <div v-if="expandedKeys.has(g.key)" class="mt-2 space-y-1 border-t pt-2">
            <div
              v-for="flow in g.flows"
              :key="flow.id"
              class="flex items-center justify-between gap-2 text-xs text-muted-foreground"
            >
              <div class="flex items-center gap-2 min-w-0">
                <span v-if="flow.domain" class="truncate">{{ flow.domain }}</span>
                <span v-else class="truncate">{{ flow.inbound || '—' }}</span>
                <span v-if="flow.source_ip" class="truncate text-muted-foreground">{{
                  flow.source_ip
                }}</span>
              </div>
              <div class="flex items-center gap-2 shrink-0">
                <span v-if="g.nodeCountry" class="flex items-center gap-1">
                  <Globe class="h-3 w-3" />
                  {{ g.nodeCountry }}
                </span>
              </div>
              <div class="flex items-center gap-2 shrink-0">
                <span class="flex items-center gap-1 text-emerald-600">
                  <ArrowUp class="h-2.5 w-2.5" />
                  {{ formatBytes(flow.upload) }}
                </span>
                <span class="flex items-center gap-1 text-blue-600">
                  <ArrowDown class="h-2.5 w-2.5" />
                  {{ formatBytes(flow.download) }}
                </span>
              </div>
            </div>
          </div>
        </CardContent>
      </UiCard>
    </div>
  </div>
</template>
