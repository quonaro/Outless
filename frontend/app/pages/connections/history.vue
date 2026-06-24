<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  Activity,
  ArrowUp,
  ArrowDown,
  Globe,
  ChevronDown,
  ChevronUp,
  Trash2,
} from 'lucide-vue-next'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import { useDomainHistory, useClearDomainHistory } from '~/composables/stats/useEntityTraffic'
import { useConnectionsSearch } from '~/composables/useConnectionsSearch'
import { useConfirm } from '~/composables/useConfirm'
import { formatBytes } from '~/utils'

const { data: historyData, isLoading: historyLoading } = useDomainHistory(30)
const { mutate: clearHistory } = useClearDomainHistory()
const { confirm } = useConfirm()
const searchQuery = useConnectionsSearch()

const expandedHistoryUsers = ref<Set<string>>(new Set())
const expandedHistoryNodes = ref<Set<string>>(new Set())

function toggleHistoryUser(userId: string) {
  const set = new Set(expandedHistoryUsers.value)
  if (set.has(userId)) {
    set.delete(userId)
  } else {
    set.add(userId)
  }
  expandedHistoryUsers.value = set
}

function toggleHistoryNode(nodeKey: string) {
  const set = new Set(expandedHistoryNodes.value)
  if (set.has(nodeKey)) {
    set.delete(nodeKey)
  } else {
    set.add(nodeKey)
  }
  expandedHistoryNodes.value = set
}

async function handleClearHistory() {
  const ok = await confirm({
    title: 'Clear Domain History',
    message: 'This will permanently delete all domain history records. Are you sure?',
    variant: 'destructive',
    confirmLabel: 'Clear',
    cancelLabel: 'Cancel',
  })
  if (ok) {
    clearHistory()
  }
}

const filteredHistoryData = computed(() => {
  if (!historyData.value?.items) return null
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return historyData.value

  const items = historyData.value.items
    .map((user) => {
      const userMatch = user.name.toLowerCase().includes(q)

      const filteredNodes = user.nodes
        .map((node) => {
          const nodeMatch = node.name.toLowerCase().includes(q)
          const filteredDomains = node.domains.filter((d) => d.name.toLowerCase().includes(q))

          if (nodeMatch) return node
          if (filteredDomains.length > 0) {
            return { ...node, domains: filteredDomains }
          }
          return null
        })
        .filter((n) => n !== null)

      if (userMatch) {
        if (filteredNodes.length > 0) {
          return { ...user, nodes: filteredNodes }
        }
        return user
      }

      if (filteredNodes.length > 0) {
        return { ...user, nodes: filteredNodes }
      }
      return null
    })
    .filter((u) => u !== null)

  return { items }
})
</script>

<template>
  <ClientOnly>
    <template #fallback>
      <div class="py-8 text-center text-muted-foreground">Loading history...</div>
    </template>

    <div class="space-y-2">
      <div class="flex justify-end">
        <button
          class="inline-flex items-center gap-1.5 rounded-md bg-destructive px-3 py-1.5 text-sm font-medium text-destructive-foreground transition-colors hover:bg-destructive/90"
          @click="handleClearHistory"
        >
          <Trash2 class="h-3.5 w-3.5" />
          Clear History
        </button>
      </div>

      <div v-if="historyLoading" class="py-8 text-center text-muted-foreground">
        Loading history...
      </div>
      <div
        v-else-if="!filteredHistoryData?.items?.length"
        class="py-8 text-center text-muted-foreground"
      >
        No domain history
      </div>
      <template v-else>
        <UiCard
          v-for="user in filteredHistoryData.items"
          :key="user.id"
          class="p-3 cursor-pointer"
          @click="toggleHistoryUser(user.id)"
        >
          <CardContent class="p-0">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-center gap-2 min-w-0">
                <Activity class="h-4 w-4 shrink-0 text-orange-500" />
                <span class="text-sm font-medium truncate">{{ user.name }}</span>
                <span
                  class="inline-flex items-center rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground"
                >
                  {{ user.nodes.length }}
                </span>
                <component
                  :is="expandedHistoryUsers.has(user.id) ? ChevronUp : ChevronDown"
                  class="h-3 w-3 text-muted-foreground"
                />
              </div>
              <div class="flex items-center gap-3 text-xs">
                <span class="flex items-center gap-1 text-emerald-600">
                  <ArrowUp class="h-3 w-3" />
                  {{ formatBytes(user.upload_bytes) }}
                </span>
                <span class="flex items-center gap-1 text-blue-600">
                  <ArrowDown class="h-3 w-3" />
                  {{ formatBytes(user.download_bytes) }}
                </span>
              </div>
            </div>

            <div v-if="expandedHistoryUsers.has(user.id)" class="mt-2 space-y-1 border-t pt-2">
              <div
                v-for="node in user.nodes"
                :key="node.id"
                class="cursor-pointer"
                @click.stop="toggleHistoryNode(`${user.id}||${node.id}`)"
              >
                <div
                  class="flex items-center justify-between gap-2 py-1 text-xs text-muted-foreground"
                >
                  <div class="flex items-center gap-2 min-w-0">
                    <Globe class="h-3 w-3 shrink-0" />
                    <span class="truncate">{{ node.name }}</span>
                    <component
                      :is="
                        expandedHistoryNodes.has(`${user.id}||${node.id}`) ? ChevronUp : ChevronDown
                      "
                      class="h-2.5 w-2.5 text-muted-foreground"
                    />
                  </div>
                  <div class="flex items-center gap-2 shrink-0">
                    <span class="flex items-center gap-1 text-emerald-600">
                      <ArrowUp class="h-2.5 w-2.5" />
                      {{ formatBytes(node.upload_bytes) }}
                    </span>
                    <span class="flex items-center gap-1 text-blue-600">
                      <ArrowDown class="h-2.5 w-2.5" />
                      {{ formatBytes(node.download_bytes) }}
                    </span>
                  </div>
                </div>

                <div
                  v-if="expandedHistoryNodes.has(`${user.id}||${node.id}`)"
                  class="mt-1 space-y-1 pl-4"
                >
                  <div
                    v-for="domain in node.domains"
                    :key="domain.id"
                    class="flex items-center justify-between gap-2 py-0.5 text-xs text-muted-foreground"
                  >
                    <div class="flex items-center gap-2 min-w-0">
                      <Globe class="h-2.5 w-2.5 shrink-0 text-blue-500" />
                      <span class="truncate">{{ domain.name }}</span>
                    </div>
                    <div class="flex items-center gap-2 shrink-0">
                      <span class="flex items-center gap-1 text-emerald-600">
                        <ArrowUp class="h-2.5 w-2.5" />
                        {{ formatBytes(domain.upload_bytes) }}
                      </span>
                      <span class="flex items-center gap-1 text-blue-600">
                        <ArrowDown class="h-2.5 w-2.5" />
                        {{ formatBytes(domain.download_bytes) }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </UiCard>
      </template>
    </div>
  </ClientOnly>
</template>
