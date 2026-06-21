<script setup lang="ts">
import { ref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { Plus } from 'lucide-vue-next'
import type { Group, CreateGroup, UpdateGroup } from '~/utils/schemas/group'
import { fetchGroups, createGroup, updateGroup, deleteGroup } from '~/utils/services/group'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'

const queryClient = useQueryClient()

const { data: groups, isLoading } = useQuery({
  queryKey: ['groups'],
  queryFn: () => fetchGroups(),
})

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const selectedGroup = ref<Group | null>(null)
const groupName = ref('')
const groupSourceURL = ref('')
const isCreateSubmitting = ref(false)
const isEditSubmitting = ref(false)

const createMutation = useMutation({
  mutationFn: (data: CreateGroup) => createGroup(data),
  onSuccess: () => {
    showCreateDialog.value = false
    groupName.value = ''
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})

const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: string; data: UpdateGroup }) => updateGroup(id, data),
  onSuccess: () => {
    showEditDialog.value = false
    groupName.value = ''
    selectedGroup.value = null
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})

const deleteMutation = useMutation({
  mutationFn: (id: string) => deleteGroup(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})

function handleCreateGroup() {
  if (!groupName.value.trim() || isCreateSubmitting.value) return
  isCreateSubmitting.value = true
  createMutation.mutate(
    { name: groupName.value, source_url: groupSourceURL.value.trim(), random_enabled: false },
    {
      onSettled: () => {
        isCreateSubmitting.value = false
      },
    }
  )
}

function handleEditGroup() {
  if (!selectedGroup.value || !groupName.value.trim() || isEditSubmitting.value) return
  isEditSubmitting.value = true
  updateMutation.mutate(
    {
      id: selectedGroup.value.id,
      data: {
        name: groupName.value,
        source_url: groupSourceURL.value.trim(),
        random_enabled: selectedGroup.value.random_enabled ?? false,
      },
    },
    {
      onSettled: () => {
        isEditSubmitting.value = false
      },
    }
  )
}

function handleDeleteGroup(group: Group) {
  if (!confirm(`Are you sure you want to delete group "${group.name}"?`)) return
  deleteMutation.mutate(group.id)
}

function openEditDialog(group: Group) {
  updateMutation.reset()
  isEditSubmitting.value = false
  selectedGroup.value = group
  groupName.value = group.name
  groupSourceURL.value = group.source_url ?? ''
  showEditDialog.value = true
}

function openCreateDialog() {
  createMutation.reset()
  isCreateSubmitting.value = false
  groupName.value = ''
  groupSourceURL.value = ''
  showCreateDialog.value = true
}

function closeCreateDialog() {
  createMutation.reset()
  isCreateSubmitting.value = false
  groupSourceURL.value = ''
  showCreateDialog.value = false
}

function closeEditDialog() {
  updateMutation.reset()
  isEditSubmitting.value = false
  showEditDialog.value = false
  selectedGroup.value = null
  groupName.value = ''
  groupSourceURL.value = ''
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex justify-end items-center">
      <UiButton @click="openCreateDialog">
        <Plus class="h-4 w-4 mr-2" />
        Create Group
      </UiButton>
    </div>

    <div v-if="isLoading" class="text-center text-muted-foreground py-8">Loading groups...</div>
    <div v-else-if="!groups || groups.length === 0" class="text-center text-muted-foreground py-8">
      No groups found
    </div>

    <UiCard v-for="group in groups" :key="group.id" class="p-4">
      <CardContent class="p-0">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <h3 class="text-lg font-semibold">{{ group.name }}</h3>
            <p class="mt-1 text-sm text-muted-foreground">ID: {{ group.id }}</p>
            <p class="text-sm text-muted-foreground">
              Created: {{ new Date(group.created_at).toLocaleString() }}
            </p>
            <p class="text-sm text-muted-foreground">Source: {{ group.source_url || 'Manual' }}</p>
          </div>
          <div class="flex shrink-0 gap-2">
            <UiButton variant="outline" size="sm" @click="openEditDialog(group)"> Edit </UiButton>
            <UiButton
              variant="destructive"
              size="sm"
              :disabled="deleteMutation.isPending"
              @click="handleDeleteGroup(group)"
            >
              Delete
            </UiButton>
          </div>
        </div>
      </CardContent>
    </UiCard>

    <Sheet v-model:open="showCreateDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Create Group</SheetTitle>
        </SheetHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">Group Name</label>
            <UiInput
              v-model="groupName"
              placeholder="Enter group name"
              @keyup.enter="handleCreateGroup"
            />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Source URL (optional)</label>
            <UiInput v-model="groupSourceURL" placeholder="https://example.com/subscription" />
          </div>
        </div>
        <SheetFooter>
          <UiButton variant="outline" @click="closeCreateDialog"> Cancel </UiButton>
          <UiButton :disabled="!groupName.trim() || isCreateSubmitting" @click="handleCreateGroup">
            {{ isCreateSubmitting ? 'Creating...' : 'Create' }}
          </UiButton>
        </SheetFooter>
      </SheetContent>
    </Sheet>

    <Sheet v-model:open="showEditDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Edit Group</SheetTitle>
        </SheetHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">Group Name</label>
            <UiInput
              v-model="groupName"
              placeholder="Enter group name"
              @keyup.enter="handleEditGroup"
            />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Source URL (optional)</label>
            <UiInput v-model="groupSourceURL" placeholder="https://example.com/subscription" />
          </div>
        </div>
        <SheetFooter>
          <UiButton variant="outline" @click="closeEditDialog"> Cancel </UiButton>
          <UiButton :disabled="!groupName.trim() || isEditSubmitting" @click="handleEditGroup">
            {{ isEditSubmitting ? 'Updating...' : 'Update' }}
          </UiButton>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  </div>
</template>
