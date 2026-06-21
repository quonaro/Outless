<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Group } from '~/utils/schemas/group'
import type { Node } from '~/utils/schemas/node'
import { useGroupNodesInfinite } from '~/composables/nodes/useGroupNodesInfinite'
import UiButton from '~/components/ui/button/button.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import { countryBadgeLabel } from '~/utils/country'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import UiInput from '~/components/ui/input/input.vue'
import UiLabel from '~/components/ui/label/label.vue'
import { ArrowRight, Copy, MoreHorizontal, Plus, Pencil, Trash2 } from 'lucide-vue-next'

const props = withDefaults(defineProps<{
  group: Group
  search: string
  isSyncing: boolean
  isCancelled: boolean
  syncError: string
  syncProgressPercent: number
  syncProcessedCount: number
  syncTotalCount: number
  syncInterrupted: boolean
  syncAddedCount: number
  deletingIds: Set<string>
  movingIds: Set<string>
  selectedIds: Set<string>
  allGroups: Group[]
  syncNodeError: (nodeId: string) => string
  editingGroup: boolean
  deletingGroup: boolean
}>(), { allGroups: () => [] })

const emit = defineEmits<{
  startSync: []
  cancelSync: []
  removeNode: [node: Node]
  editGroup: [group: { id: string, name: string, source_url: string, random_enabled: boolean, random_limit: number | null }]
  deleteGroup: [groupId: string]
  addNode: [groupId: string]
  moveNode: [payload: { node: Node, targetGroupId: string }]
  toggleSelection: [nodeId: string]
  bulkMove: [targetGroupId: string]
  bulkDelete: []
  duplicateNode: [node: Node]
}>()

const copiedNodeIDs = ref<Set<string>>(new Set())
const moveNodeDialogOpen = ref(false)
const moveNodeTarget = ref<Node | null>(null)
const moveTargetGroupId = ref('')
const accordionOpen = ref(false)
const editDialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const editName = ref('')
const editSourceUrl = ref('')
const editRandomEnabled = ref(false)
const editRandomLimit = ref<string>('')
const canSaveEdit = computed(() => editName.value.trim().length > 0 && !props.editingGroup)

const accordionStorageKey = computed(() => `outless:nodes:group-accordion:${props.group.id}`)

onMounted(() => {
  if (!import.meta.client) return
  const saved = localStorage.getItem(accordionStorageKey.value)
  if (saved === '1') accordionOpen.value = true
  if (saved === '0') accordionOpen.value = false
})

const {
  data: nodePages,
  fetchNextPage,
  hasNextPage,
  isFetchingNextPage,
  isLoading,
} = useGroupNodesInfinite(() => props.group.id)

const allNodesInGroup = computed(() =>
  nodePages.value?.pages.flatMap((p) => p.nodes) ?? [],
)

const displayNodes = computed(() => {
  let list = allNodesInGroup.value
  const q = props.search.trim().toLowerCase()
  if (q) {
    list = list.filter((n) =>
      `${n.url} ${n.id} ${n.country}`.toLowerCase().includes(q),
    )
  }
  return list
})

const scrollRoot = ref<HTMLElement | null>(null)
const loadSentinel = ref<HTMLElement | null>(null)
let listObserver: IntersectionObserver | null = null

function maybeLoadMoreInList() {
  if (!hasNextPage.value || isFetchingNextPage.value) return
  void fetchNextPage()
}

watch(
  [scrollRoot, loadSentinel, () => hasNextPage.value],
  () => {
    listObserver?.disconnect()
    listObserver = null
    const root = scrollRoot.value
    const target = loadSentinel.value
    if (!root || !target) return
    listObserver = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          maybeLoadMoreInList()
        }
      },
      { root, rootMargin: '160px 0px 160px 0px', threshold: 0 },
    )
    listObserver.observe(target)
  },
  { flush: 'post' },
)

onBeforeUnmount(() => {
  listObserver?.disconnect()
  listObserver = null
})


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

const emptyMessage = computed(() => {
  if (isLoading.value && allNodesInGroup.value.length === 0) return 'Loading nodes…'
  if (allNodesInGroup.value.length === 0) return 'No nodes in this group'
  return 'No nodes match the current filters'
})

function onAccordionToggle(ev: Event) {
  const target = ev.currentTarget as HTMLDetailsElement | null
  if (!target) return
  accordionOpen.value = target.open
}

watch(accordionOpen, (value) => {
  if (!import.meta.client) return
  localStorage.setItem(accordionStorageKey.value, value ? '1' : '0')
})

function openEditDialog() {
  editName.value = props.group.name
  editSourceUrl.value = props.group.source_url ?? ''
  editRandomEnabled.value = props.group.random_enabled ?? false
  editRandomLimit.value = props.group.random_limit?.toString() ?? ''
  editDialogOpen.value = true
}

function confirmEdit() {
  emit('editGroup', {
    id: props.group.id,
    name: editName.value.trim(),
    source_url: editSourceUrl.value?.trim() || '',
    random_enabled: editRandomEnabled.value,
    random_limit: editRandomLimit.value ? parseInt(editRandomLimit.value) : null,
  })
  editDialogOpen.value = false
}

function openDeleteDialog() {
  deleteDialogOpen.value = true
}

function confirmDelete() {
  emit('deleteGroup', props.group.id)
  deleteDialogOpen.value = false
}

function openMoveNodeDialog(node: Node) {
  moveNodeTarget.value = node
  moveTargetGroupId.value = node.group_id ?? ''
  moveNodeDialogOpen.value = true
}

function confirmMoveNode() {
  if (!moveNodeTarget.value) return
  emit('moveNode', { node: moveNodeTarget.value, targetGroupId: moveTargetGroupId.value })
  moveNodeDialogOpen.value = false
  moveNodeTarget.value = null
  moveTargetGroupId.value = ''
}
function handleDuplicateNode() {
  if (!moveNodeTarget.value) return
  emit('duplicateNode', moveNodeTarget.value)
  moveNodeDialogOpen.value = false
  moveNodeTarget.value = null
  moveTargetGroupId.value = ''
}
</script>

<template>
  <details class="rounded-md border bg-card" :open="accordionOpen" @toggle="onAccordionToggle">
    <summary class="cursor-pointer list-none bg-muted/25 px-4 py-3">
      <div class="flex min-w-0 items-start justify-between gap-2">
        <div class="min-w-0 flex-1">
          <p class="truncate font-medium">
            {{ props.group.name }} <span class="text-muted-foreground">({{ props.group.total_nodes }})</span>
          </p>
          <div class="flex items-center gap-2 text-xs text-muted-foreground">
            <p class="truncate min-w-0">
              {{ props.group.source_url || 'Manual group' }}
            </p>
            <span v-if="props.group.last_synced_at" class="shrink-0"> · Last sync: {{ new Date(props.group.last_synced_at).toLocaleString() }}</span>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-1">
          <UiButton variant="ghost" size="icon" class="h-8 w-8" @click.prevent="emit('addNode', props.group.id)">
            <Plus class="h-4 w-4" />
          </UiButton>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <UiButton variant="ghost" size="icon" class="h-8 w-8" @click.prevent>
                <MoreHorizontal class="h-4 w-4" />
              </UiButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem @click.prevent="openEditDialog" :disabled="props.editingGroup">
                <Pencil class="mr-2 h-4 w-4" />
                Edit
              </DropdownMenuItem>
              <DropdownMenuItem class="text-destructive focus:text-destructive" @click.prevent="openDeleteDialog" :disabled="props.deletingGroup">
                <Trash2 class="mr-2 h-4 w-4" />
                Delete group
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </summary>
    <div
      v-if="props.isSyncing || props.syncInterrupted"
      class="flex items-center gap-2 border-b border-border/80 bg-muted/40 px-4 py-2.5"
    >
      <span class="text-xs font-medium text-muted-foreground">{{ props.isSyncing ? 'Load' : 'Load interrupted' }}</span>
      <span class="rounded-full border border-primary/40 bg-primary/10 px-2.5 py-0.5 font-mono text-xs tabular-nums text-primary">
        {{ props.syncProgressPercent }}% · {{ props.syncProcessedCount }}/{{ props.syncTotalCount }}
      </span>
      <span class="rounded-full border border-emerald-500/40 bg-emerald-500/10 px-2 py-0.5 text-xs font-medium text-emerald-700">
        +{{ props.syncAddedCount }} new
      </span>
    </div>


    <CardContent class="border-t px-0 py-0">
      <div v-if="props.isCancelled" class="flex flex-wrap items-center gap-2 px-4 pt-3">
        <p class="text-xs text-amber-600">Sync cancelled</p>
      </div>

      <p v-if="props.syncError" class="px-4 pt-2 text-xs text-red-500">
        {{ props.syncError }}
      </p>

      <div
        ref="scrollRoot"
        class="max-h-[min(70vh,28rem)] overflow-y-auto overscroll-contain rounded-md border border-border/60 bg-muted/20 px-4 py-3 pr-2 [scrollbar-width:thin] [scrollbar-color:rgba(148,163,184,0.45)_transparent] [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-zinc-600/60 hover:[&::-webkit-scrollbar-thumb]:bg-zinc-500/80"
      >
        <div class="space-y-2">
          <UiCard
            v-for="node in displayNodes"
            :key="node.id"
            class="px-3 py-2"
          >
            <CardContent class="p-0">
              <div class="flex items-center gap-2">
                <input
                  type="checkbox"
                  :checked="props.selectedIds.has(node.id)"
                  @change="emit('toggleSelection', node.id)"
                  class="h-4 w-4 rounded border-gray-400 shrink-0"
                >
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
                    {{ node.id }}
                  </p>
                  <p class="mt-1 text-xs">
                    <span
                      class="ml-2 inline-flex items-center rounded-full border border-border/80 bg-muted/35 px-2 py-0.5 text-xs font-medium tabular-nums text-muted-foreground"
                    >
                      {{ countryBadgeLabel(node.country) }}
                    </span>
                    <span v-if="props.syncNodeError(node.id)" class="ml-2 text-red-600">
                      {{ props.syncNodeError(node.id) }}
                    </span>
                  </p>
                </div>
                <div class="flex shrink-0 flex-nowrap items-center justify-end gap-1 whitespace-nowrap">
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                      <UiButton variant="ghost" size="icon" class="h-7 w-7" @click.prevent>
                        <MoreHorizontal class="h-3.5 w-3.5" />
                      </UiButton>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem @click.prevent="openMoveNodeDialog(node)" :disabled="props.movingIds.has(node.id)">
                        <ArrowRight class="mr-2 h-3.5 w-3.5" />
                        Move
                      </DropdownMenuItem>
                      <DropdownMenuItem @click.prevent="copyNodeURL(node)">
                        <Copy class="mr-2 h-3.5 w-3.5" />
                        Copy
                      </DropdownMenuItem>
                      <DropdownMenuItem class="text-destructive focus:text-destructive" @click.prevent="emit('removeNode', node)" :disabled="props.deletingIds.has(node.id)">
                        <Trash2 class="mr-2 h-3.5 w-3.5" />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            </CardContent>
          </UiCard>

          <div ref="loadSentinel" class="h-2 shrink-0" aria-hidden="true" />

          <p v-if="isFetchingNextPage" class="py-1 text-center text-xs text-muted-foreground">
            Loading more…
          </p>

          <p
            v-if="displayNodes.length === 0"
            class="py-6 text-center text-sm text-muted-foreground"
          >
            {{ emptyMessage }}
          </p>
        </div>
      </div>
    </CardContent>
  </details>

  <!-- Edit Group Dialog -->
  <Dialog v-model:open="editDialogOpen">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Edit Group</DialogTitle>
        <DialogDescription>
          Change the group name and public URL.
        </DialogDescription>
      </DialogHeader>
      <div class="space-y-4 py-4">
        <div class="space-y-2">
          <UiLabel for="edit-name">Name</UiLabel>
          <UiInput
            id="edit-name"
            v-model="editName"
            placeholder="Group name"
          />
        </div>
        <div class="space-y-2">
          <UiLabel for="edit-source-url">Public URL</UiLabel>
          <UiInput
            id="edit-source-url"
            v-model="editSourceUrl"
            placeholder="https://example.com/nodes.txt"
          />
        </div>
        <div class="flex items-center gap-2">
          <input
            id="edit-random-enabled"
            v-model="editRandomEnabled"
            type="checkbox"
            class="h-4 w-4 rounded border-input"
          >
          <label for="edit-random-enabled" class="text-sm">Random selection for subscriptions</label>
        </div>
        <div class="space-y-2">
          <UiLabel for="edit-random-limit">Limit (optional)</UiLabel>
          <UiInput
            id="edit-random-limit"
            v-model="editRandomLimit"
            type="number"
            min="1"
            placeholder="Max nodes to return"
          />
          <p class="text-xs text-muted-foreground">Maximum number of nodes to return in subscriptions</p>
        </div>
      </div>
      <DialogFooter>
        <UiButton variant="outline" @click="editDialogOpen = false">Cancel</UiButton>
        <UiButton @click="confirmEdit" :disabled="!canSaveEdit">
          {{ props.editingGroup ? 'Saving...' : 'Save' }}
        </UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- Delete Group Dialog -->
  <Dialog :open="deleteDialogOpen" @update:open="deleteDialogOpen = $event">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Delete group</DialogTitle>
        <DialogDescription>
          Are you sure you want to delete group "{{ props.group.name }}"? This action cannot be undone.
        </DialogDescription>
      </DialogHeader>
      <DialogFooter>
        <UiButton variant="outline" @click="deleteDialogOpen = false">Cancel</UiButton>
        <UiButton variant="destructive" @click="confirmDelete" :disabled="props.deletingGroup">
          Delete
        </UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog :open="moveNodeDialogOpen" @update:open="moveNodeDialogOpen = $event">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Move node</DialogTitle>
        <DialogDescription>
          Select target group to move this node.
        </DialogDescription>
      </DialogHeader>
      <div class="py-4">
        <UiLabel for="move-target-group">Target group</UiLabel>
        <select id="move-target-group" v-model="moveTargetGroupId" class="mt-1.5 w-full rounded-md border bg-background px-3 py-2 text-sm">
          <option value="">No group</option>
          <option v-for="g in props.allGroups" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
      </div>
      <DialogFooter>
        <UiButton variant="outline" @click="handleDuplicateNode">Duplicate</UiButton>
        <UiButton variant="outline" @click="moveNodeDialogOpen = false">Cancel</UiButton>
        <UiButton @click="confirmMoveNode">Move</UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
