<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import type { Group } from '~/utils/schemas/group'
import type { Node } from '~/utils/schemas/node'
import { deleteNode } from '~/utils/services/node'
import { deleteGroup, updateGroup } from '~/utils/services/group'
import { useGroupSync } from '~/composables/groups/useGroupSync'
import GroupAccordionItem from '~/components/GroupAccordionItem.vue'
import { onSSEOpen } from '~/composables/useSSE'

const props = defineProps<{
  groups: Group[]
  search: string
  selectedNodeIds: Set<string>
}>()

const emit = defineEmits<{
  removeNode: [node: Node]
  addNode: [groupId: string]
  moveNode: [payload: { node: Node; targetGroupId: string }]
  toggleSelection: [nodeId: string]
  duplicateNode: [node: Node]
}>()

const queryClient = useQueryClient()
const syncStates: Record<string, ReturnType<typeof useGroupSync>> = {}
const deletingNodeIDs = ref<Set<string>>(new Set())
const movingNodeIDs = ref<Set<string>>(new Set())
const deletingGroupIDs = ref<Set<string>>(new Set())
const editingGroupIDs = ref<Set<string>>(new Set())

const visibleGroups = computed(() => {
  const q = props.search.trim().toLowerCase()
  if (!q) return props.groups
  return props.groups.filter((g) => `${g.name} ${g.id} ${g.source_url}`.toLowerCase().includes(q))
})

const deleteMutation = useMutation({
  mutationFn: (id: string) => deleteNode(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})
const deleteGroupMutation = useMutation({
  mutationFn: (groupId: string) => deleteGroup(groupId),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
  },
})
const editGroupMutation = useMutation({
  mutationFn: ({
    id,
    name,
    source_url,
    random_enabled,
    random_limit,
  }: {
    id: string
    name: string
    source_url: string
    random_enabled: boolean
    random_limit?: number | null
  }) => updateGroup(id, { name, source_url, random_enabled, random_limit }),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['groups'] }),
})
function stateForGroup(groupID: string) {
  if (!syncStates[groupID]) {
    syncStates[groupID] = useGroupSync(groupID)
    syncStates[groupID].requestSyncState()
  }
  return syncStates[groupID]
}

function syncIsRunning(groupId: string): boolean {
  return stateForGroup(groupId).isSyncing.value
}
function syncCancelled(groupId: string): boolean {
  return stateForGroup(groupId).isCancelled.value
}
function syncErrorText(groupId: string): string {
  return stateForGroup(groupId).error.value
}

function syncProcessedCount(groupId: string): number {
  return stateForGroup(groupId).syncProcessed.value
}
function syncAddedCount(groupId: string): number {
  return stateForGroup(groupId).syncAddedCount.value
}

function syncTotalCount(group: Group): number {
  const total = stateForGroup(group.id).syncTotal.value
  return total > 0 ? total : (group.total_nodes ?? 0)
}

function syncProgressPercent(group: Group): number {
  const total = syncTotalCount(group)
  if (total <= 0) return 0
  const processed = syncProcessedCount(group.id)
  const pct = Math.floor((processed / total) * 100)
  if (processed > 0 && pct === 0) return 1
  return pct
}

function syncInterrupted(group: Group): boolean {
  const st = stateForGroup(group.id)
  return (
    !st.isSyncing.value && st.syncTotal.value > 0 && st.syncProcessed.value < st.syncTotal.value
  )
}

function startSync(group: Group) {
  stateForGroup(group.id).startSync()
}
function cancelSync(group: Group) {
  stateForGroup(group.id).cancelSync()
}
function removeNode(node: Node) {
  if (!confirm(`Delete node ${node.id}?`)) return
  const next = new Set(deletingNodeIDs.value)
  next.add(node.id)
  deletingNodeIDs.value = next
  deleteMutation.mutate(node.id, {
    onSettled: () => {
      deletingNodeIDs.value.delete(node.id)
    },
  })
}
function handleAddNode(groupId: string) {
  emit('addNode', groupId)
}
function handleMoveNode(payload: { node: Node; targetGroupId: string }) {
  emit('moveNode', payload)
}
function nodeSyncError(groupID: string, nodeID: string): string {
  return stateForGroup(groupID).syncingNodes.value.get(nodeID)?.error ?? ''
}
function handleEditGroup(group: {
  id: string
  name: string
  source_url: string
  random_enabled: boolean
  random_limit: number | null
}) {
  const existingGroup = props.groups.find((g) => g.id === group.id)
  if (!existingGroup) return

  const next = new Set(editingGroupIDs.value)
  next.add(group.id)
  editingGroupIDs.value = next
  editGroupMutation.mutate(
    {
      id: group.id,
      name: group.name,
      source_url: group.source_url,
      random_enabled: group.random_enabled,
      random_limit: group.random_limit,
    },
    {
      onSettled: () => {
        const current = new Set(editingGroupIDs.value)
        current.delete(group.id)
        editingGroupIDs.value = current
      },
    }
  )
}
function handleDeleteGroup(groupId: string) {
  const next = new Set(deletingGroupIDs.value)
  next.add(groupId)
  deletingGroupIDs.value = next
  deleteGroupMutation.mutate(groupId, {
    onSettled: () => {
      const current = new Set(deletingGroupIDs.value)
      current.delete(groupId)
      deletingGroupIDs.value = current
    },
  })
}

let stopResyncOnWsOpen: (() => void) | null = null
onMounted(() => {
  if (!import.meta.client) return
  stopResyncOnWsOpen = onSSEOpen(() => {
    for (const id of Object.keys(syncStates)) {
      const st = syncStates[id]
      if (!st) continue
      st.requestSyncState()
    }
  })
})
onBeforeUnmount(() => {
  stopResyncOnWsOpen?.()
  stopResyncOnWsOpen = null
})
</script>

<template>
  <div class="space-y-3">
    <GroupAccordionItem
      v-for="group in visibleGroups"
      :key="group.id"
      :group="group"
      :search="props.search"
      :moving-ids="movingNodeIDs"
      :selected-ids="props.selectedNodeIds"
      :all-groups="props.groups"
      :is-syncing="syncIsRunning(group.id)"
      :is-cancelled="syncCancelled(group.id)"
      :sync-error="syncErrorText(group.id)"
      :sync-progress-percent="syncProgressPercent(group)"
      :sync-processed-count="syncProcessedCount(group.id)"
      :sync-total-count="syncTotalCount(group)"
      :sync-interrupted="syncInterrupted(group)"
      :sync-added-count="syncAddedCount(group.id)"
      :editing-group="editingGroupIDs.has(group.id)"
      :deleting-group="deletingGroupIDs.has(group.id)"
      :deleting-ids="deletingNodeIDs"
      :sync-node-error="(id: string) => nodeSyncError(group.id, id)"
      @add-node="handleAddNode"
      @move-node="handleMoveNode"
      @toggle-selection="emit('toggleSelection', $event)"
      @duplicate-node="emit('duplicateNode', $event)"
      @start-sync="startSync(group)"
      @cancel-sync="cancelSync(group)"
      @remove-node="removeNode"
      @edit-group="handleEditGroup"
      @delete-group="handleDeleteGroup"
    />
  </div>
</template>
