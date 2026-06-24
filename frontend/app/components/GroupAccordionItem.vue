<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Group } from '~/utils/schemas/group'
import type { Node } from '~/utils/schemas/node'
import { useGroupNodesInfinite } from '~/composables/nodes/useGroupNodesInfinite'
import UiButton from '~/components/ui/button/button.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import NodeCard from '~/components/NodeCard.vue'
import { useInbounds } from '~/composables/inbounds/useInbounds'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '~/components/ui/sheet'
import UiInput from '~/components/ui/input/input.vue'
import UiLabel from '~/components/ui/label/label.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import { Plus, Pencil, Trash2, MoreHorizontal } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    group: Group
    search: string
    deletingIds: Set<string>
    movingIds: Set<string>
    selectedIds: Set<string>
    allGroups: Group[]
    editingGroup: boolean
    deletingGroup: boolean
  }>(),
  { allGroups: () => [] }
)

const { data: inbounds } = useInbounds()

const emit = defineEmits<{
  removeNode: [node: Node]
  editGroup: [
    group: {
      id: string
      name: string
      random_enabled: boolean
      random_limit: number | null
    },
  ]
  deleteGroup: [groupId: string]
  addNode: [groupId: string]
  moveNode: [payload: { node: Node; targetGroupId: string }]
  toggleSelection: [nodeId: string]
  bulkMove: [targetGroupId: string]
  bulkDelete: []
  updateNodeGroups: [nodeId: string, groupIds: string[]]
}>()

const moveNodeDialogOpen = ref(false)
const moveNodeTarget = ref<Node | null>(null)
const moveTargetGroupId = ref('')
const accordionOpen = ref(false)
const editDialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const editName = ref('')
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

const allNodesInGroup = computed(() => nodePages.value?.pages.flatMap((p) => p.nodes) ?? [])

const displayNodes = computed(() => {
  let list = allNodesInGroup.value
  const q = props.search.trim().toLowerCase()
  if (q) {
    list = list.filter((n) => `${n.url} ${n.id} ${n.country}`.toLowerCase().includes(q))
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
      { root, rootMargin: '160px 0px 160px 0px', threshold: 0 }
    )
    listObserver.observe(target)
  },
  { flush: 'post' }
)

onBeforeUnmount(() => {
  listObserver?.disconnect()
  listObserver = null
})

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
  editRandomEnabled.value = props.group.random_enabled ?? false
  editRandomLimit.value = props.group.random_limit?.toString() ?? ''
  editDialogOpen.value = true
}

function confirmEdit() {
  emit('editGroup', {
    id: props.group.id,
    name: editName.value.trim(),
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
  moveTargetGroupId.value = node.group_ids[0] ?? ''
  moveNodeDialogOpen.value = true
}

function confirmMoveNode() {
  if (!moveNodeTarget.value) return
  emit('moveNode', { node: moveNodeTarget.value, targetGroupId: moveTargetGroupId.value })
  moveNodeDialogOpen.value = false
  moveNodeTarget.value = null
  moveTargetGroupId.value = ''
}

const editGroupsDialogOpen = ref(false)
const editGroupsTarget = ref<Node | null>(null)
const editGroupsSelected = ref<string[]>([])

function openEditGroupsDialog(node: Node) {
  editGroupsTarget.value = node
  editGroupsSelected.value = [...node.group_ids]
  editGroupsDialogOpen.value = true
}

function confirmEditGroups() {
  if (!editGroupsTarget.value) return
  emit('updateNodeGroups', editGroupsTarget.value.id, editGroupsSelected.value)
  editGroupsDialogOpen.value = false
  editGroupsTarget.value = null
  editGroupsSelected.value = []
}
</script>

<template>
  <details class="rounded-md border bg-card" :open="accordionOpen" @toggle="onAccordionToggle">
    <summary class="cursor-pointer list-none bg-muted/25 px-4 py-3">
      <div class="flex min-w-0 items-start justify-between gap-2">
        <div class="min-w-0 flex-1">
          <p class="truncate font-medium">
            {{ props.group.name }}
            <span class="text-muted-foreground">({{ props.group.total_nodes }})</span>
          </p>
        </div>
        <div class="flex shrink-0 items-center gap-1">
          <UiButton
            variant="ghost"
            size="icon"
            class="h-8 w-8"
            @click.prevent="emit('addNode', props.group.id)"
          >
            <Plus class="h-4 w-4" />
          </UiButton>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <UiButton variant="ghost" size="icon" class="h-8 w-8" @click.prevent>
                <MoreHorizontal class="h-4 w-4" />
              </UiButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem :disabled="props.editingGroup" @click.prevent="openEditDialog">
                <Pencil class="mr-2 h-4 w-4" />
                Edit
              </DropdownMenuItem>
              <DropdownMenuItem
                class="text-destructive focus:text-destructive"
                :disabled="props.deletingGroup"
                @click.prevent="openDeleteDialog"
              >
                <Trash2 class="mr-2 h-4 w-4" />
                Delete group
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </summary>
    <CardContent class="border-t px-0 py-0">
      <div
        ref="scrollRoot"
        class="max-h-[min(70vh,28rem)] overflow-y-auto overscroll-contain rounded-md border border-border/60 bg-muted/20 px-4 py-3 pr-2 [scrollbar-width:thin] [scrollbar-color:rgba(148,163,184,0.45)_transparent] [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-zinc-600/60 hover:[&::-webkit-scrollbar-thumb]:bg-zinc-500/80"
      >
        <div class="space-y-2 pb-3">
          <div v-for="node in displayNodes" :key="node.id" class="flex items-start gap-2">
            <input
              type="checkbox"
              :checked="props.selectedIds.has(node.id)"
              class="h-4 w-4 rounded border-gray-400 shrink-0 mt-2"
              @change="emit('toggleSelection', node.id)"
            />
            <NodeCard
              class="flex-1"
              :node="node"
              :inbounds="inbounds ?? []"
              show-actions
              :deleting="props.deletingIds.has(node.id)"
              :moving="props.movingIds.has(node.id)"
              @move-node="openMoveNodeDialog"
              @edit-groups="openEditGroupsDialog"
              @delete-node="emit('removeNode', $event)"
            />
          </div>

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

  <!-- Edit Group Sheet -->
  <Sheet v-model:open="editDialogOpen">
    <SheetContent>
      <SheetHeader>
        <SheetTitle>Edit Group</SheetTitle>
        <SheetDescription> Change the group name. </SheetDescription>
      </SheetHeader>
      <div class="space-y-4 py-4">
        <div class="space-y-2">
          <UiLabel for="edit-name">Name</UiLabel>
          <UiInput id="edit-name" v-model="editName" placeholder="Group name" />
        </div>
        <div class="flex items-center gap-2">
          <input
            id="edit-random-enabled"
            v-model="editRandomEnabled"
            type="checkbox"
            class="h-4 w-4 rounded border-input"
          />
          <label for="edit-random-enabled" class="text-sm"
            >Random selection for subscriptions</label
          >
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
          <p class="text-xs text-muted-foreground">
            Maximum number of nodes to return in subscriptions
          </p>
        </div>
      </div>
      <SheetFooter>
        <UiButton variant="outline" @click="editDialogOpen = false">Cancel</UiButton>
        <UiButton :disabled="!canSaveEdit" @click="confirmEdit">
          {{ props.editingGroup ? 'Saving...' : 'Save' }}
        </UiButton>
      </SheetFooter>
    </SheetContent>
  </Sheet>

  <!-- Delete Group Dialog -->
  <Dialog :open="deleteDialogOpen" @update:open="deleteDialogOpen = $event">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Delete group</DialogTitle>
        <DialogDescription>
          Are you sure you want to delete group "{{ props.group.name }}"? This action cannot be
          undone.
        </DialogDescription>
      </DialogHeader>
      <DialogFooter>
        <UiButton variant="outline" @click="deleteDialogOpen = false">Cancel</UiButton>
        <UiButton variant="destructive" :disabled="props.deletingGroup" @click="confirmDelete">
          Delete
        </UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog :open="moveNodeDialogOpen" @update:open="moveNodeDialogOpen = $event">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Move node</DialogTitle>
        <DialogDescription> Select target group to move this node. </DialogDescription>
      </DialogHeader>
      <div class="py-4">
        <UiLabel for="move-target-group">Target group</UiLabel>
        <select
          id="move-target-group"
          v-model="moveTargetGroupId"
          class="mt-1.5 w-full rounded-md border bg-background px-3 py-2 text-sm"
        >
          <option value="">No group</option>
          <option v-for="g in props.allGroups" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
      </div>
      <DialogFooter>
        <UiButton variant="outline" @click="moveNodeDialogOpen = false">Cancel</UiButton>
        <UiButton @click="confirmMoveNode">Move</UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog :open="editGroupsDialogOpen" @update:open="editGroupsDialogOpen = $event">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Edit groups</DialogTitle>
        <DialogDescription> Select groups this node belongs to. </DialogDescription>
      </DialogHeader>
      <div class="py-4">
        <div class="max-h-60 overflow-y-auto space-y-2">
          <label
            v-for="g in props.allGroups"
            :key="g.id"
            class="flex items-center gap-2 text-sm cursor-pointer"
          >
            <input
              v-model="editGroupsSelected"
              type="checkbox"
              :value="g.id"
              class="h-4 w-4 rounded border-input"
            />
            <span>{{ g.name }}</span>
          </label>
        </div>
      </div>
      <DialogFooter>
        <UiButton variant="outline" @click="editGroupsDialogOpen = false">Cancel</UiButton>
        <UiButton @click="confirmEditGroups">Save</UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
