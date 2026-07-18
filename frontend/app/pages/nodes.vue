<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { Pencil, Hash, Monitor, Link, Tags } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiButton from '~/components/ui/button/button.vue'
import NodeTable from '~/components/NodeTable.vue'
import UiInput from '~/components/ui/input/input.vue'
import NodeLifetimeInput from '~/components/NodeLifetimeInput.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'
import { useInfiniteNodes } from '~/composables/nodes/useInfiniteNodes'
import { useGroups } from '~/composables/groups/useGroups'
import type { Node } from '~/utils/schemas/node'
import type { TopUpInput } from '~/utils/schemas/group'
import { createNode, deleteNode, updateNode, batchDeleteNodes } from '~/utils/services/node'
import { createGroup, defaultTopUpForm, buildTopUpInput } from '~/utils/services/group'
import type { TopUpFormValues } from '~/utils/services/group'
import { resolveCreateNodeErrorMessage } from '~/utils/node'
import TopUpFields from '~/components/TopUpFields.vue'
import { useInbounds } from '~/composables/inbounds/useInbounds'
import VlessUrlPreview from '~/components/VlessUrlPreview.vue'
import UiSelect from '~/components/ui/select/select.vue'
import EditNodeDialog from '~/components/EditNodeDialog.vue'
import ImportNodesMenu from '~/components/ImportNodesMenu.vue'
import GroupsTopUpStatusDialog from '~/components/GroupsTopUpStatusDialog.vue'
import CreateNodesMenu from '~/components/CreateNodesMenu.vue'

definePageMeta({ layout: 'default' })

useHead({
  title: 'Nodes',
})

const queryClient = useQueryClient()
const { confirm } = useConfirm()
const { data: inbounds } = useInbounds()
const groupFilter = ref<string>('')

const {
  data: nodePages,
  isLoading: nodesLoading,
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
} = useInfiniteNodes(
  computed(() => true),
  groupFilter
)
const { data: groups, isLoading: groupsLoading } = useGroups()
const showInitialNodesShell = computed(
  () =>
    (groupsLoading.value && groups.value == null) || (nodesLoading.value && nodePages.value == null)
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
const nodeExpiresAt = ref<string | undefined>(undefined)
const createNodeErrorMessage = ref('')
const isCreateGroupSubmitting = ref(false)
const isCreateNodeSubmitting = ref(false)
const showTopUp = ref(false)
const topUpForm = ref<TopUpFormValues>(defaultTopUpForm())
const deletingNodeIDs = ref<Set<string>>(new Set())
const selectedNodeIDs = ref<Set<string>>(new Set())

const showEditNodeDialog = ref(false)
const editNodeTarget = ref<Node | null>(null)

const createGroupMutation = useMutation({
  mutationFn: (payload: {
    name: string
    random_enabled: boolean
    random_limit: number | null
    top_up?: TopUpInput
  }) => createGroup(payload),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    showCreateGroupDialog.value = false
    groupNameInput.value = ''
    groupRandomEnabledInput.value = false
    groupRandomLimitInput.value = ''
    showTopUp.value = false
    topUpForm.value = defaultTopUpForm()
  },
})

const createNodeMutation = useMutation({
  mutationFn: (payload: {
    url: string
    group_ids: string[]
    is_self: boolean
    expires_at?: string
  }) => createNode(payload),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    showCreateNodeDialog.value = false
    nodeURLInput.value = ''
    nodeGroupIDsInput.value = []
    nodeExpiresAt.value = undefined
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
  if (!searchValue) return list
  return list.filter((node) => {
    const groupNames = node.group_ids.map((id) => groupNameByID.value[id] ?? '').join(' ')
    return `${node.url} ${node.id} ${node.country} ${node.country_code ?? ''} ${node.country_name ?? ''} ${node.country_flag ?? ''} ${groupNames}`
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
      top_up: showTopUp.value ? buildTopUpInput(topUpForm.value) : undefined,
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
    {
      url: isSelf ? '' : url,
      group_ids: groupIds,
      is_self: isSelf,
      expires_at: nodeExpiresAt.value,
    },
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

function closeCreateNodeDialog() {
  showCreateNodeDialog.value = false
  nodeIsSelfInput.value = false
  nodeGroupIDsInput.value = []
  nodeExpiresAt.value = undefined
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

function openEditNodeDialog(node: Node) {
  editNodeTarget.value = node
  showEditNodeDialog.value = true
}

async function submitEditNode(payload: { url?: string; groupIds: string[]; expiresAt?: string }) {
  if (!editNodeTarget.value) return
  const nodeId = editNodeTarget.value.id
  const body: { url?: string; group_ids?: string[]; expires_at?: string } = {
    group_ids: payload.groupIds,
  }
  if (payload.url) body.url = payload.url
  if (payload.expiresAt !== undefined) body.expires_at = payload.expiresAt || undefined
  try {
    await updateNode(nodeId, body)
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    showEditNodeDialog.value = false
    editNodeTarget.value = null
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to update node', { description: msg })
  }
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
  if (hasNextPage.value && !isFetchingNextPage.value) {
    fetchNextPage()
  }
}

function handleImportFinished() {
  queryClient.invalidateQueries({ queryKey: ['nodes'] })
  queryClient.invalidateQueries({ queryKey: ['groups'] })
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
            <CreateNodesMenu
              :groups="groups"
              @create-group="showCreateGroupDialog = true"
              @create-node="showCreateNodeDialog = true"
            />
            <GroupsTopUpStatusDialog :groups="groups" />
            <ImportNodesMenu @imported="handleImportFinished" />
          </div>
          <UiInput
            id="node-search"
            v-model="search"
            name="node-search"
            placeholder="Search by URL, ID, group..."
            class="w-full sm:max-w-md"
          />
          <UiSelect
            v-model="groupFilter"
            :options="[
              { label: 'All groups', value: '' },
              ...(groups ?? []).map((g) => ({ label: g.name, value: g.id })),
            ]"
            class="w-full sm:w-48"
          />
        </div>

        <div v-if="showInitialNodesShell" class="py-8 text-center text-muted-foreground">
          Loading data...
        </div>

        <NodeTable
          v-else
          :nodes="filteredFlatNodes"
          :inbounds="inbounds ?? []"
          :group-name-by-i-d="groupNameByID"
          :selected-node-i-ds="selectedNodeIDs"
          :deleting-node-i-ds="deletingNodeIDs"
          @toggle-selection="handleToggleSelection"
          @delete-node="removeNode"
          @edit-node="openEditNodeDialog"
        />

        <div ref="loadMoreAnchor" class="h-10 text-center text-xs text-muted-foreground">
          <span v-if="isFetchingNextPage">Loading more nodes...</span>
        </div>
      </div>

      <Sheet v-model:open="showCreateGroupDialog">
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Create Group</SheetTitle>
            <SheetDescription>Create a new group for organizing nodes.</SheetDescription>
          </SheetHeader>
          <div class="grow min-h-0 space-y-4 overflow-y-auto py-4">
            <div class="space-y-2">
              <label
                class="inline-flex items-center gap-2 text-sm font-medium"
                for="create-group-name"
              >
                <Pencil class="h-4 w-4" />
                Name
              </label>
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
              <label
                class="inline-flex items-center gap-2 text-sm font-medium"
                for="create-group-random-limit"
                ><Hash class="h-4 w-4" /> Limit (optional)</label
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
            <div class="flex items-center gap-2">
              <input
                id="node-create-top-up"
                v-model="showTopUp"
                type="checkbox"
                class="h-4 w-4 rounded border-input"
              />
              <label for="node-create-top-up" class="text-sm">Self-refilling (top-up) group</label>
            </div>

            <TopUpFields v-if="showTopUp" v-model="topUpForm" />
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
          <div class="grow min-h-0 space-y-4 overflow-y-auto py-4">
            <div v-if="!hasSelfNode" class="flex items-center gap-2">
              <input
                id="create-node-is-self"
                v-model="nodeIsSelfInput"
                type="checkbox"
                class="h-4 w-4 rounded border-input"
              />
              <label
                for="create-node-is-self"
                class="inline-flex items-center gap-2 text-sm font-medium"
                ><Monitor class="h-4 w-4" /> Use Current Machine</label
              >
            </div>
            <div v-if="!nodeIsSelfInput" class="space-y-2">
              <label
                class="inline-flex items-center gap-2 text-sm font-medium"
                for="create-node-url"
              >
                <Link class="h-4 w-4" />
                VLESS URL
              </label>
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
              <label class="inline-flex items-center gap-2 text-sm font-medium">
                <Tags class="h-4 w-4" />
                Groups
              </label>
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
            <NodeLifetimeInput v-model="nodeExpiresAt" />
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

      <EditNodeDialog
        v-model="showEditNodeDialog"
        :node="editNodeTarget"
        :groups="groups ?? []"
        @save="submitEditNode"
      />
    </ClientOnly>
  </UiPageLayout>
</template>
