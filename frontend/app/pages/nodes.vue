<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { Plus, Server } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'
import GroupAccordion from '~/components/GroupAccordion.vue'
import { useInfiniteNodes } from '~/composables/nodes/useInfiniteNodes'
import { useGroups } from '~/composables/groups/useGroups'
import type { Node } from '~/utils/schemas/node'
import { createNode, deleteNode, updateNode } from '~/utils/services/node'
import { createGroup } from '~/utils/services/group'

definePageMeta({ layout: 'default' })

useHead({
  title: 'Nodes',
})

type ViewMode = 'grouped' | 'flat'

const queryClient = useQueryClient()
const viewMode = ref<ViewMode>('grouped')

const {
  data: nodePages,
  isLoading: nodesLoading,
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
} = useInfiniteNodes(computed(() => viewMode.value === 'flat'))
const { data: groups, isLoading: groupsLoading } = useGroups()
/** Full-page skeleton: flat mode waits for global infinite list; grouped mode only waits for groups. */
const showInitialNodesShell = computed(
  () =>
    (groupsLoading.value && groups.value == null) ||
    (viewMode.value === 'flat' && nodesLoading.value && nodePages.value == null)
)
const loadMoreAnchor = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null
let stopLoadMoreAnchorWatch: (() => void) | null = null

/** Nodes loaded via global infinite scroll (flat list / partial cache). */
const infiniteNodesFlat = computed<Node[]>(
  () => nodePages.value?.pages.flatMap((page) => page.nodes) ?? []
)

const search = ref('')

const showCreateGroupDialog = ref(false)
const showCreateNodeDialog = ref(false)
const groupNameInput = ref('')
const groupSourceURLInput = ref('')
const groupRandomEnabledInput = ref(false)
const groupRandomLimitInput = ref<string>('')
const nodeURLInput = ref('')
const nodeGroupIDInput = ref('')
const createNodeErrorMessage = ref('')
const isCreateGroupSubmitting = ref(false)
const isCreateNodeSubmitting = ref(false)
const deletingNodeIDs = ref<Set<string>>(new Set())
const selectedNodeIDs = ref<Set<string>>(new Set())
const bulkMoveDialogOpen = ref(false)
const bulkMoveTargetGroupId = ref('')

const createGroupMutation = useMutation({
  mutationFn: (payload: {
    name: string
    source_url: string
    random_enabled: boolean
    random_limit: number | null
  }) => createGroup(payload),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    showCreateGroupDialog.value = false
    groupNameInput.value = ''
    groupSourceURLInput.value = ''
    groupRandomEnabledInput.value = false
    groupRandomLimitInput.value = ''
  },
})

const createNodeMutation = useMutation({
  mutationFn: (payload: { url: string; group_id: string }) => createNode(payload),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    showCreateNodeDialog.value = false
    nodeURLInput.value = ''
    nodeGroupIDInput.value = ''
    createNodeErrorMessage.value = ''
  },
})

const deleteNodeMutation = useMutation({
  mutationFn: (id: string) => deleteNode(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})

const copiedNodeIDs = ref<Set<string>>(new Set())

const groupNameByID = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  for (const group of groups.value ?? []) {
    map[group.id] = group.name
  }
  return map
})

const filteredFlatNodes = computed<Node[]>(() => {
  const list = infiniteNodesFlat.value
  const searchValue = search.value.trim().toLowerCase()
  return list.filter((node) => {
    if (!searchValue) return true
    const groupName = groupNameByID.value[node.group_id] ?? ''
    return `${node.url} ${node.id} ${node.country} ${groupName}`.toLowerCase().includes(searchValue)
  })
})

function submitCreateGroup() {
  const name = groupNameInput.value.trim()
  const sourceURL = groupSourceURLInput.value.trim()
  if (!name || isCreateGroupSubmitting.value) return
  isCreateGroupSubmitting.value = true
  createGroupMutation.mutate(
    {
      name,
      source_url: sourceURL,
      random_enabled: groupRandomEnabledInput.value,
      random_limit: (() => {
        if (!groupRandomLimitInput.value) return null
        const n = parseInt(groupRandomLimitInput.value)
        return Number.isNaN(n) || n <= 0 ? null : n
      })(),
    },
    {
      onSettled: () => {
        isCreateGroupSubmitting.value = false
      },
    }
  )
}

function submitCreateNode() {
  const url = nodeURLInput.value.trim()
  const groupId = nodeGroupIDInput.value.trim()
  if (!url || !groupId || isCreateNodeSubmitting.value) {
    createNodeErrorMessage.value = 'Please select a group.'
    return
  }
  createNodeErrorMessage.value = ''
  isCreateNodeSubmitting.value = true
  createNodeMutation.mutate(
    { url, group_id: groupId },
    {
      onError: (error) => {
        createNodeErrorMessage.value = resolveCreateNodeErrorMessage(error)
      },
      onSettled: () => {
        isCreateNodeSubmitting.value = false
      },
    }
  )
}

function resolveCreateNodeErrorMessage(error: unknown): string {
  const statusCode = Number((error as { statusCode?: unknown })?.statusCode)
  if (statusCode === 409) {
    return 'Node with this URL already exists.'
  }

  const data = (error as { data?: unknown })?.data as
    | { message?: unknown; detail?: unknown; title?: unknown }
    | undefined
  if (typeof data?.message === 'string' && data.message.trim()) return data.message
  if (typeof data?.detail === 'string' && data.detail.trim()) return data.detail
  if (typeof data?.title === 'string' && data.title.trim()) return data.title

  const message = (error as { message?: unknown })?.message
  if (typeof message === 'string' && message.trim()) return message
  return 'Failed to create node.'
}

function handleAddNode(groupId: string) {
  nodeGroupIDInput.value = groupId
  showCreateNodeDialog.value = true
}

function closeCreateNodeDialog() {
  showCreateNodeDialog.value = false
  createNodeErrorMessage.value = ''
}

const movingNodeIDs = ref<Set<string>>(new Set())

function handleMoveNode(payload: { node: Node; targetGroupId: string }) {
  const next = new Set(movingNodeIDs.value)
  next.add(payload.node.id)
  movingNodeIDs.value = next
  updateNode(payload.node.id, {
    url: payload.node.url,
    group_id: payload.targetGroupId,
  })
    .then(() => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] })
      queryClient.invalidateQueries({ queryKey: ['groups'] })
    })
    .finally(() => {
      const current = new Set(movingNodeIDs.value)
      current.delete(payload.node.id)
      movingNodeIDs.value = current
    })
}

function handleToggleSelection(nodeId: string) {
  const next = new Set(selectedNodeIDs.value)
  if (next.has(nodeId)) {
    next.delete(nodeId)
  } else {
    next.add(nodeId)
  }
  selectedNodeIDs.value = next
}

function openBulkMoveDialog() {
  bulkMoveTargetGroupId.value = ''
  bulkMoveDialogOpen.value = true
}

function handleBulkMove() {
  const promises = Array.from(selectedNodeIDs.value).map((nodeId) =>
    updateNode(nodeId, { group_id: bulkMoveTargetGroupId.value })
  )
  Promise.all(promises)
    .then(() => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] })
      queryClient.invalidateQueries({ queryKey: ['groups'] })
      selectedNodeIDs.value = new Set()
      bulkMoveDialogOpen.value = false
    })
    .catch((err: Error) => {
      toast.error('Bulk move failed', { description: err.message })
    })
}

function handleBulkDelete() {
  if (!confirm(`Delete ${selectedNodeIDs.value.size} selected nodes?`)) return
  const promises = Array.from(selectedNodeIDs.value).map((nodeId) => deleteNode(nodeId))
  Promise.all(promises)
    .then(() => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] })
      queryClient.invalidateQueries({ queryKey: ['groups'] })
      selectedNodeIDs.value = new Set()
    })
    .catch((err: Error) => {
      toast.error('Bulk delete failed', { description: err.message })
    })
}

function handleDuplicateNode(node: Node) {
  createNode({ url: node.url, group_id: node.group_id }).then(() => {
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  })
}

function removeNode(node: Node) {
  if (!confirm(`Delete node ${node.id}?`)) return
  const next = new Set(deletingNodeIDs.value)
  next.add(node.id)
  deletingNodeIDs.value = next
  deleteNodeMutation.mutate(node.id, {
    onSettled: () => {
      const current = new Set(deletingNodeIDs.value)
      current.delete(node.id)
      deletingNodeIDs.value = current
    },
  })
}

async function copyNodeURL(node: Node) {
  await navigator.clipboard.writeText(node.url)
  const next = new Set(copiedNodeIDs.value)
  next.add(node.id)
  copiedNodeIDs.value = next
  setTimeout(() => {
    const current = new Set(copiedNodeIDs.value)
    current.delete(node.id)
    copiedNodeIDs.value = current
  }, 1200)
}

function maybeLoadMore() {
  if (viewMode.value !== 'flat') return
  if (hasNextPage.value && !isFetchingNextPage.value) {
    fetchNextPage()
  }
}

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      const hit = entries.some((entry) => entry.isIntersecting)
      if (hit) {
        maybeLoadMore()
      }
    },
    { rootMargin: '250px 0px 250px 0px' }
  )

  stopLoadMoreAnchorWatch = watch(
    loadMoreAnchor,
    (el) => {
      if (!observer) return
      observer.disconnect()
      if (el) {
        observer.observe(el)
      }
    },
    { flush: 'post', immediate: true }
  )
})

onBeforeUnmount(() => {
  stopLoadMoreAnchorWatch?.()
  stopLoadMoreAnchorWatch = null
  if (observer) {
    observer.disconnect()
    observer = null
  }
})
</script>

<template>
  <UiPageLayout title="Nodes" description="Manage your proxy nodes">
    <ClientOnly>
      <template #fallback>
        <div class="py-8 text-center text-muted-foreground">Loading nodes...</div>
      </template>

      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <UiButton variant="outline" @click="viewMode = 'grouped'">Grouped</UiButton>
            <UiButton variant="outline" @click="viewMode = 'flat'">Flat</UiButton>
          </div>
          <div v-if="selectedNodeIDs.size > 0" class="flex items-center gap-2">
            <span class="text-sm font-medium">{{ selectedNodeIDs.size }} selected</span>
            <UiButton size="sm" variant="outline" @click="openBulkMoveDialog"> Move </UiButton>
            <UiButton size="sm" variant="destructive" @click="handleBulkDelete"> Delete </UiButton>
            <UiButton size="sm" variant="ghost" @click="selectedNodeIDs = new Set()">
              Clear
            </UiButton>
          </div>
          <div v-else class="flex flex-wrap items-center gap-2">
            <UiButton @click="showCreateGroupDialog = true">
              <Plus class="h-4 w-4 mr-2" />
              Create Group
            </UiButton>
            <UiButton @click="showCreateNodeDialog = true">
              <Server class="h-4 w-4 mr-2" />
              Create Node
            </UiButton>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UiInput
            id="node-search"
            v-model="search"
            name="node-search"
            placeholder="Search by URL, ID, country, group..."
            class="w-full sm:max-w-md"
          />
        </div>

        <div v-if="showInitialNodesShell" class="py-8 text-center text-muted-foreground">
          Loading data...
        </div>

        <GroupAccordion
          v-else-if="viewMode === 'grouped'"
          :groups="groups ?? []"
          :search="search"
          :selected-node-ids="selectedNodeIDs"
          @add-node="handleAddNode"
          @move-node="handleMoveNode"
          @toggle-selection="handleToggleSelection"
          @duplicate-node="handleDuplicateNode"
        />

        <div v-else class="space-y-2">
          <UiCard v-for="node in filteredFlatNodes" :key="node.id" class="px-3 py-2">
            <CardContent class="p-0">
              <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div class="min-w-0 flex-1">
                  <div class="group relative min-w-0">
                    <p class="truncate text-sm font-medium">{{ node.url }}</p>
                    <div
                      class="pointer-events-none absolute left-0 top-full z-20 mt-1 hidden max-h-48 w-[min(90vw,40rem)] overflow-y-auto whitespace-pre-wrap break-all rounded-md border bg-popover px-2 py-1 text-xs text-popover-foreground shadow-md group-hover:block"
                    >
                      {{ node.url }}
                    </div>
                  </div>
                  <p class="text-xs text-muted-foreground">
                    {{ node.id }} ·
                    {{ groupNameByID[node.group_id] ?? (node.group_id || 'No group') }}
                    ·
                    <span
                      class="ml-1 inline-flex items-center rounded-full border border-border/80 bg-muted/40 px-2 py-0.5 tabular-nums"
                      >{{ countryBadgeLabel(node.country) }}</span
                    >
                  </p>
                </div>
                <div class="flex shrink-0 flex-wrap gap-1 sm:flex-nowrap">
                  <UiButton
                    variant="outline"
                    size="sm"
                    class="whitespace-nowrap"
                    @click="copyNodeURL(node)"
                  >
                    {{ copiedNodeIDs.has(node.id) ? 'Copied' : 'Copy' }}
                  </UiButton>
                  <UiButton
                    variant="destructive"
                    size="sm"
                    class="whitespace-nowrap"
                    :disabled="deletingNodeIDs.has(node.id)"
                    @click="removeNode(node)"
                  >
                    {{ deletingNodeIDs.has(node.id) ? 'Deleting...' : 'Delete' }}
                  </UiButton>
                </div>
              </div>
            </CardContent>
          </UiCard>
        </div>

        <div
          v-if="viewMode === 'flat'"
          ref="loadMoreAnchor"
          class="h-10 text-center text-xs text-muted-foreground"
        >
          <span v-if="isFetchingNextPage">Loading more nodes...</span>
        </div>
      </div>

      <Sheet v-model:open="showCreateGroupDialog">
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Create Group</SheetTitle>
            <SheetDescription>Create a new group for organizing nodes.</SheetDescription>
          </SheetHeader>
          <div class="space-y-4 py-4">
            <div class="space-y-2">
              <label class="text-sm font-medium" for="create-group-name">Name</label>
              <UiInput
                id="create-group-name"
                v-model="groupNameInput"
                name="create-group-name"
                placeholder="Group name"
                @keyup.enter="submitCreateGroup"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium" for="create-group-source-url"
                >Source URL (optional)</label
              >
              <UiInput
                id="create-group-source-url"
                v-model="groupSourceURLInput"
                name="create-group-source-url"
                placeholder="https://example.com/subscription"
              />
            </div>
            <div class="flex items-center gap-2">
              <input
                id="create-group-random-enabled"
                v-model="groupRandomEnabledInput"
                type="checkbox"
                class="h-4 w-4 rounded border-input"
              />
              <label for="create-group-random-enabled" class="text-sm"
                >Random selection for subscriptions</label
              >
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium" for="create-group-random-limit"
                >Limit (optional)</label
              >
              <UiInput
                id="create-group-random-limit"
                v-model="groupRandomLimitInput"
                type="number"
                min="1"
                placeholder="Max nodes to return"
              />
              <p class="text-xs text-muted-foreground">
                Maximum number of nodes to return in subscriptions
              </p>
            </div>
          </div>
          <SheetFooter>
            <UiButton variant="outline" @click="showCreateGroupDialog = false">Cancel</UiButton>
            <UiButton
              :disabled="!groupNameInput.trim() || isCreateGroupSubmitting"
              @click="submitCreateGroup"
            >
              {{ isCreateGroupSubmitting ? 'Creating...' : 'Create' }}
            </UiButton>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Sheet v-model:open="showCreateNodeDialog">
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Create Node</SheetTitle>
            <SheetDescription>Add a new VLESS node to a group.</SheetDescription>
          </SheetHeader>
          <div class="space-y-4 py-4">
            <div class="space-y-2">
              <label class="text-sm font-medium" for="create-node-url">VLESS URL</label>
              <UiInput
                id="create-node-url"
                v-model="nodeURLInput"
                name="create-node-url"
                placeholder="vless://uuid@host:443?..."
                @keyup.enter="submitCreateNode"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium" for="create-node-group-id">Group</label>
              <select
                id="create-node-group-id"
                v-model="nodeGroupIDInput"
                name="create-node-group-id"
                class="w-full rounded-md border bg-background px-3 py-2 text-sm"
                required
              >
                <option value="" disabled selected>Select a group</option>
                <option v-for="group in groups ?? []" :key="group.id" :value="group.id">
                  {{ group.name }}
                </option>
              </select>
            </div>
            <p v-if="createNodeErrorMessage" class="text-sm text-red-600 dark:text-red-400">
              {{ createNodeErrorMessage }}
            </p>
          </div>
          <SheetFooter>
            <UiButton variant="outline" @click="closeCreateNodeDialog"> Cancel </UiButton>
            <UiButton
              :disabled="!nodeURLInput.trim() || !nodeGroupIDInput.trim() || isCreateNodeSubmitting"
              @click="submitCreateNode"
            >
              {{ isCreateNodeSubmitting ? 'Creating...' : 'Create' }}
            </UiButton>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Sheet v-model:open="bulkMoveDialogOpen">
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Move selected nodes</SheetTitle>
            <SheetDescription>Move selected nodes to another group.</SheetDescription>
          </SheetHeader>
          <div class="space-y-4 py-4">
            <p class="text-sm text-muted-foreground">
              Move {{ selectedNodeIDs.size }} selected nodes to another group.
            </p>
            <div class="space-y-2">
              <label class="text-sm font-medium" for="bulk-move-target-group">Target group</label>
              <select
                id="bulk-move-target-group"
                v-model="bulkMoveTargetGroupId"
                class="w-full rounded-md border bg-background px-3 py-2 text-sm"
              >
                <option value="">No group</option>
                <option v-for="group in groups ?? []" :key="group.id" :value="group.id">
                  {{ group.name }}
                </option>
              </select>
            </div>
          </div>
          <SheetFooter>
            <UiButton variant="outline" @click="bulkMoveDialogOpen = false">Cancel</UiButton>
            <UiButton :disabled="!bulkMoveTargetGroupId" @click="handleBulkMove">Move</UiButton>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </ClientOnly>
  </UiPageLayout>
</template>
