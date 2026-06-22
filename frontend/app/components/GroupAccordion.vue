<script setup lang="ts">
import { ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import type { Group } from '~/utils/schemas/group'
import type { Node } from '~/utils/schemas/node'
import { deleteNode } from '~/utils/services/node'
import { deleteGroup, updateGroup } from '~/utils/services/group'
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

const visibleGroups = computed(() => {
  const q = props.search.trim().toLowerCase()
  if (!q) return props.groups
  return props.groups.filter((g) => `${g.name} ${g.id}`.toLowerCase().includes(q))
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
    random_enabled,
    random_limit,
  }: {
    id: string
    name: string
    random_enabled: boolean
    random_limit?: number | null
  }) => updateGroup(id, { name, random_enabled, random_limit }),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['groups'] }),
})
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
function handleEditGroup(group: {
  id: string
  name: string
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
      :editing-group="editingGroupIDs.has(group.id)"
      :deleting-group="deletingGroupIDs.has(group.id)"
      :deleting-ids="deletingNodeIDs"
      @add-node="handleAddNode"
      @move-node="handleMoveNode"
      @toggle-selection="emit('toggleSelection', $event)"
      @duplicate-node="emit('duplicateNode', $event)"
      @remove-node="removeNode"
      @edit-group="handleEditGroup"
      @delete-group="handleDeleteGroup"
    />
  </div>
</template>
