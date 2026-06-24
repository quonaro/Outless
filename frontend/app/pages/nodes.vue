<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { Plus, Server } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiButton from '~/components/ui/button/button.vue'
import NodeCard from '~/components/NodeCard.vue'
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
import { createNode, deleteNode, updateNode, batchDeleteNodes } from '~/utils/services/node'
import { createGroup } from '~/utils/services/group'
import { useInbounds } from '~/composables/inbounds/useInbounds'
import VlessUrlPreview from '~/components/VlessUrlPreview.vue'

definePageMeta({ layout: 'default' })

useHead({
  title: 'Nodes',
})

type ViewMode = 'grouped' | 'flat'

const queryClient = useQueryClient()
const { confirm } = useConfirm()
const { data: inbounds } = useInbounds()
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

const hasSelfNode = computed<boolean>(() => {
  const list = infiniteNodesFlat.value
  return list.some((node) => node.is_self)
})

const showCreateGroupDialog = ref(false)
const showCreateNodeDialog = ref(false)
const groupNameInput = ref('')
const groupRandomEnabledInput = ref(false)
const groupRandomLimitInput = ref<string>('')
const nodeURLInput = ref('')
const nodeGroupIDsInput = ref<string[]>([])
const nodeIsSelfInput = ref(false)
const createNodeErrorMessage = ref('')
const isCreateGroupSubmitting = ref(false)
const isCreateNodeSubmitting = ref(false)
const deletingNodeIDs = ref<Set<string>>(new Set())
const selectedNodeIDs = ref<Set<string>>(new Set())
const editingNodeGroupIDs = ref<Set<string>>(new Set())

const createGroupMutation = useMutation({
  mutationFn: (payload: { name: string; random_enabled: boolean; random_limit: number | null }) =>
    createGroup(payload),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    showCreateGroupDialog.value = false
    groupNameInput.value = ''
    groupRandomEnabledInput.value = false
    groupRandomLimitInput.value = ''
  },
})

const createNodeMutation = useMutation({
  mutationFn: (payload: { url: string; group_ids: string[]; is_self: boolean }) =>
    createNode(payload),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    showCreateNodeDialog.value = false
    nodeURLInput.value = ''
    nodeGroupIDsInput.value = []
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
    const groupNames = node.group_ids.map((id) => groupNameByID.value[id] ?? '').join(' ')
    return `${node.url} ${node.id} ${node.country} ${groupNames}`
      .toLowerCase()
      .includes(searchValue)
  })
})

function submitCreateGroup() {
  const name = groupNameInput.value.trim()
  if (!name || isCreateGroupSubmitting.value) return
  isCreateGroupSubmitting.value = true
  createGroupMutation.mutate(
    {
      name,
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
  const groupIds = nodeGroupIDsInput.value
  const isSelf = nodeIsSelfInput.value
  if ((!isSelf && !url) || groupIds.length === 0 || isCreateNodeSubmitting.value) {
    createNodeErrorMessage.value = 'Please select at least one group.'
    return
  }
  createNodeErrorMessage.value = ''
  isCreateNodeSubmitting.value = true
  createNodeMutation.mutate(
    { url: isSelf ? '' : url, group_ids: groupIds, is_self: isSelf },
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
    return 'A node with this identifier already exists.'
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
  nodeGroupIDsInput.value = [groupId]
  nodeIsSelfInput.value = false
  showCreateNodeDialog.value = true
}

function closeCreateNodeDialog() {
  showCreateNodeDialog.value = false
  nodeIsSelfInput.value = false
  nodeGroupIDsInput.value = []
  createNodeErrorMessage.value = ''
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

async function handleBulkDelete() {
  const ok = await confirm({
    title: 'Bulk delete',
    message: `Delete ${selectedNodeIDs.value.size} selected nodes?`,
    variant: 'destructive',
  })
  if (!ok) return
  try {
    await batchDeleteNodes(Array.from(selectedNodeIDs.value))
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    selectedNodeIDs.value = new Set()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Bulk delete failed', { description: msg })
  }
}

function handleUpdateNodeGroups(nodeId: string, groupIds: string[]) {
  const next = new Set(editingNodeGroupIDs.value)
  next.add(nodeId)
  editingNodeGroupIDs.value = next
  updateNode(nodeId, { group_ids: groupIds })
    .then(() => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] })
      queryClient.invalidateQueries({ queryKey: ['groups'] })
    })
    .catch((err: Error) => {
      toast.error('Failed to update groups', { description: err.message })
    })
    .finally(() => {
      const current = new Set(editingNodeGroupIDs.value)
      current.delete(nodeId)
      editingNodeGroupIDs.value = current
    })
}

async function removeNode(node: Node) {
  const ok = await confirm({
    title: 'Delete node',
    message: `Delete node ${node.id}?`,
    variant: 'destructive',
  })
  if (!ok) return
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
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div v-if="selectedNodeIDs.size > 0" class="flex items-center gap-2">
            <span class="text-sm font-medium">{{ selectedNodeIDs.size }} selected</span>
            <UiButton size="sm" variant="destructive" @click="handleBulkDelete"> Delete </UiButton>
            <UiButton size="sm" variant="outline" @click="selectedNodeIDs = new Set()">
              Clear
            </UiButton>
          </div>
          <div v-else class="flex flex-wrap items-center gap-2">
            <UiButton @click="showCreateGroupDialog = true">
              <Plus class="h-4 w-4 mr-2" />
              Create Group
            </UiButton>
            <UiButton
              :disabled="!groups?.length"
              :title="groups?.length ? '' : 'Create a group first'"
              @click="showCreateNodeDialog = true"
            >
              <Server class="h-4 w-4 mr-2" />
              Create Node
            </UiButton>
          </div>
          <UiInput
            id="node-search"
            v-model="search"
            name="node-search"
            placeholder="Search by URL, ID, group..."
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
          @toggle-selection="handleToggleSelection"
          @update-node-groups="handleUpdateNodeGroups"
        />

        <div v-else class="space-y-2">
          <div v-for="node in filteredFlatNodes" :key="node.id" class="flex items-start gap-2">
            <NodeCard
              class="flex-1"
              :node="node"
              :inbounds="inbounds ?? []"
              :group-label="
                node.group_ids.length
                  ? node.group_ids.map((id) => groupNameByID[id] ?? id).join(', ')
                  : 'No group'
              "
            />
            <div class="flex shrink-0 flex-wrap gap-1 sm:flex-nowrap pt-2">
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
            <div v-if="!hasSelfNode" class="flex items-center gap-2">
              <input
                id="create-node-is-self"
                v-model="nodeIsSelfInput"
                type="checkbox"
                class="h-4 w-4 rounded border-input"
              />
              <label for="create-node-is-self" class="text-sm font-medium"
                >Use Current Machine</label
              >
            </div>
            <div v-if="!nodeIsSelfInput" class="space-y-2">
              <label class="text-sm font-medium" for="create-node-url">VLESS URL</label>
              <UiInput
                id="create-node-url"
                v-model="nodeURLInput"
                name="create-node-url"
                placeholder="vless://uuid@host:443?..."
                @keyup.enter="submitCreateNode"
              />
              <VlessUrlPreview :url="nodeURLInput" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Groups</label>
              <div
                class="max-h-32 overflow-y-auto rounded-md border bg-background px-3 py-2 text-sm space-y-1"
              >
                <label
                  v-for="group in groups ?? []"
                  :key="group.id"
                  class="flex items-center gap-2 cursor-pointer"
                >
                  <input
                    v-model="nodeGroupIDsInput"
                    type="checkbox"
                    :value="group.id"
                    class="h-4 w-4 rounded border-input"
                  />
                  <span>{{ group.name }}</span>
                </label>
              </div>
            </div>
            <p v-if="createNodeErrorMessage" class="text-sm text-red-600 dark:text-red-400">
              {{ createNodeErrorMessage }}
            </p>
          </div>
          <SheetFooter>
            <UiButton variant="outline" @click="closeCreateNodeDialog"> Cancel </UiButton>
            <UiButton
              :disabled="
                (!nodeIsSelfInput && !nodeURLInput.trim()) ||
                nodeGroupIDsInput.length === 0 ||
                isCreateNodeSubmitting
              "
              @click="submitCreateNode"
            >
              {{ isCreateNodeSubmitting ? 'Creating...' : 'Create' }}
            </UiButton>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </ClientOnly>
  </UiPageLayout>
</template>
