<script setup lang="ts">
import { computed, ref } from 'vue'
import { toast } from 'vue-sonner'
import { useQueryClient } from '@tanstack/vue-query'
import type { CreateNode, Node, UpdateNode } from '~/utils/schemas/node'
import { useNodes } from '~/composables/nodes/useNodes'
import { useCreateNode } from '~/composables/nodes/useCreateNode'
import { useUpdateNode } from '~/composables/nodes/useUpdateNode'
import { useDeleteNode } from '~/composables/nodes/useDeleteNode'
import { useGroups } from '~/composables/groups/useGroups'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardHeader from '~/components/ui/card/CardHeader.vue'
import CardTitle from '~/components/ui/card/CardTitle.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import CardFooter from '~/components/ui/card/CardFooter.vue'

const queryClient = useQueryClient()

const { data: nodes, isLoading } = useNodes()
const { data: groups } = useGroups()

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const selectedNode = ref<Node | null>(null)
const nodeUrl = ref('')
const nodeGroupId = ref('')
const filterGroupId = ref<string>('')
const isCreateSubmitting = ref(false)
const isEditSubmitting = ref(false)

const invalidate = () => queryClient.invalidateQueries({ queryKey: ['nodes'] })

const createMutation = useCreateNode({
  onSuccess: () => {
    toast.success('Node created successfully')
    showCreateDialog.value = false
    resetForm()
    invalidate()
  },
  onError: (err) => {
    if (err.message.includes('duplicate key') || err.message.includes('already exists')) {
      toast.error('Failed to create node', {
        description: 'Node with this URL already exists',
      })
    } else {
      toast.error('Failed to create node', {
        description: err.message,
      })
    }
  },
})

const updateMutation = useUpdateNode({
  onSuccess: () => {
    toast.success('Node updated successfully')
    showEditDialog.value = false
    resetForm()
    invalidate()
  },
  onError: (err) => {
    toast.error('Failed to update node', {
      description: err.message,
    })
  },
})

const deleteMutation = useDeleteNode({
  onSuccess: () => {
    toast.success('Node deleted successfully')
    invalidate()
  },
  onError: (err) => {
    toast.error('Failed to delete node', {
      description: err.message,
    })
  },
})

const visibleNodes = computed<Node[]>(() => {
  const list = nodes.value ?? []
  if (!filterGroupId.value) return list
  if (filterGroupId.value === '__ungrouped__') {
    return list.filter((n) => !n.group_id)
  }
  return list.filter((n) => n.group_id === filterGroupId.value)
})

const groupNameById = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  for (const group of groups.value ?? []) {
    map[group.id] = group.name
  }
  return map
})

function resetForm() {
  nodeUrl.value = ''
  nodeGroupId.value = ''
  selectedNode.value = null
  isCreateSubmitting.value = false
  isEditSubmitting.value = false
}

function openCreateDialog() {
  createMutation.reset()
  resetForm()
  showCreateDialog.value = true
}

function closeCreateDialog() {
  createMutation.reset()
  showCreateDialog.value = false
  resetForm()
}

function openEditDialog(node: Node) {
  updateMutation.reset()
  isEditSubmitting.value = false
  selectedNode.value = node
  nodeUrl.value = node.url
  nodeGroupId.value = node.group_id
  showEditDialog.value = true
}

function closeEditDialog() {
  updateMutation.reset()
  showEditDialog.value = false
  resetForm()
}

function handleCreate() {
  if (!nodeUrl.value.trim() || isCreateSubmitting.value) return
  const payload: CreateNode = {
    url: nodeUrl.value.trim(),
    group_id: nodeGroupId.value,
  }
  isCreateSubmitting.value = true
  createMutation.mutate(payload, {
    onSettled: () => {
      isCreateSubmitting.value = false
    },
  })
}

function handleUpdate() {
  if (!selectedNode.value || !nodeUrl.value.trim() || !nodeGroupId.value || isEditSubmitting.value) return
  const payload: UpdateNode = {
    url: nodeUrl.value.trim(),
    group_id: nodeGroupId.value,
  }
  isEditSubmitting.value = true
  updateMutation.mutate({ id: selectedNode.value.id, ...payload }, {
    onSettled: () => {
      isEditSubmitting.value = false
    },
  })
}

function handleDelete(node: Node) {
  if (!confirm(`Delete node ${node.url}?`)) return
  deleteMutation.mutate(node.id)
}

</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-end gap-3">
      <div class="flex items-center gap-2">
        <select
          v-model="filterGroupId"
          class="rounded-md border bg-background px-3 py-2 text-sm"
        >
          <option value="">All groups</option>
          <option v-for="g in groups ?? []" :key="g.id" :value="g.id">
            {{ g.name }}
          </option>
        </select>
        <UiButton @click="openCreateDialog">
          Add Node
        </UiButton>
      </div>
    </div>

    <div v-if="isLoading" class="py-8 text-center text-muted-foreground">
      Loading nodes...
    </div>
    <div
      v-else-if="visibleNodes.length === 0"
      class="py-8 text-center text-muted-foreground"
    >
      No nodes found
    </div>

    <UiCard v-for="node in visibleNodes" :key="node.id" class="p-4">
      <CardContent class="p-0">
        <div class="flex items-start justify-between gap-4">
          <div class="max-w-[52%] min-w-0 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-sm font-medium">
                {{ node.country || 'XX' }}
              </span>
            </div>
            <p class="truncate font-mono text-sm" :title="node.url">
              {{ node.url }}
            </p>
            <p class="text-xs text-muted-foreground">
              Group:
              <span class="font-medium">
                {{ groupNameById[node.group_id] ?? node.group_id }}
              </span>
              · ID: {{ node.id }}
            </p>
          </div>
          <div class="flex shrink-0 flex-nowrap gap-2">
            <UiButton variant="outline" size="sm" class="whitespace-nowrap" @click="openEditDialog(node)">
              Edit
            </UiButton>
            <UiButton
              variant="destructive"
              size="sm"
              class="whitespace-nowrap"
              :disabled="deleteMutation.isPending"
              @click="handleDelete(node)"
            >
              Delete
            </UiButton>
          </div>
        </div>
      </CardContent>
    </UiCard>

    <div
      v-if="showCreateDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <UiCard class="w-full max-w-md p-6">
        <CardHeader>
          <CardTitle>Add Node</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">VLESS URL</label>
            <UiInput
              v-model="nodeUrl"
              placeholder="vless://uuid@host:443?..."
              @keyup.enter="handleCreate"
            />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Group *</label>
            <select
              v-model="nodeGroupId"
              class="w-full rounded-md border bg-background px-3 py-2 text-sm"
            >
              <option v-for="g in groups ?? []" :key="g.id" :value="g.id">
                {{ g.name }}
              </option>
            </select>
          </div>
        </CardContent>
        <CardFooter class="flex justify-end gap-2">
          <UiButton variant="outline" @click="closeCreateDialog">
            Cancel
          </UiButton>
          <UiButton
            :disabled="!nodeUrl.trim() || !nodeGroupId || isCreateSubmitting"
            @click="handleCreate"
          >
            {{ isCreateSubmitting ? 'Adding...' : 'Add' }}
          </UiButton>
        </CardFooter>
      </UiCard>
    </div>

    <div
      v-if="showEditDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <UiCard class="w-full max-w-md p-6">
        <CardHeader>
          <CardTitle>Edit Node</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">VLESS URL</label>
            <UiInput v-model="nodeUrl" @keyup.enter="handleUpdate" />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Group *</label>
            <select
              v-model="nodeGroupId"
              class="w-full rounded-md border bg-background px-3 py-2 text-sm"
            >
              <option v-for="g in groups ?? []" :key="g.id" :value="g.id">
                {{ g.name }}
              </option>
            </select>
          </div>
        </CardContent>
        <CardFooter class="flex justify-end gap-2">
          <UiButton variant="outline" @click="closeEditDialog">
            Cancel
          </UiButton>
          <UiButton
            :disabled="!nodeUrl.trim() || isEditSubmitting"
            @click="handleUpdate"
          >
            {{ isEditSubmitting ? 'Saving...' : 'Save' }}
          </UiButton>
        </CardFooter>
      </UiCard>
    </div>
  </div>
</template>
