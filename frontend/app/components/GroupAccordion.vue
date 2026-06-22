<script setup lang="ts">
import { ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import type { Group } from '~/utils/schemas/group'
import type { Node } from '~/utils/schemas/node'
import { deleteNode } from '~/utils/services/node'
import { deleteGroup, updateGroup, syncGroup, cancelGroupSync } from '~/utils/services/group'
import GroupAccordionItem from '~/components/GroupAccordionItem.vue'

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
const deletingNodeIDs = ref<Set<string>>(new Set())
const movingNodeIDs = ref<Set<string>>(new Set())
const deletingGroupIDs = ref<Set<string>>(new Set())
const editingGroupIDs = ref<Set<string>>(new Set())
const syncingGroupIDs = ref<Set<string>>(new Set())

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
function startSync(group: Group) {
  syncingGroupIDs.value.add(group.id)
  syncMutation.mutate(group.id)
}
function cancelSync(group: Group) {
  cancelSyncMutation.mutate(group.id)
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
const syncMutation = useMutation({
  mutationFn: (id: string) => syncGroup(id),
  onSuccess: (_data, id) => {
    syncingGroupIDs.value.delete(id)
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
    queryClient.invalidateQueries({ queryKey: ['nodes', 'infinite'] })
  },
  onError: (_err, id) => {
    syncingGroupIDs.value.delete(id)
  },
})

const cancelSyncMutation = useMutation({
  mutationFn: (id: string) => cancelGroupSync(id),
  onSettled: (_data, _err, id) => {
    syncingGroupIDs.value.delete(id)
  },
})

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
      :is-syncing="syncingGroupIDs.has(group.id)"
      :editing-group="editingGroupIDs.has(group.id)"
      :deleting-group="deletingGroupIDs.has(group.id)"
      :deleting-ids="deletingNodeIDs"
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
