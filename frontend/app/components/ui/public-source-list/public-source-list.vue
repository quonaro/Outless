<script setup lang="ts">
import { ref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import type { PublicSource, CreatePublicSource, UpdatePublicSource } from '~/utils/schemas/public-source'
import { fetchPublicSources, createPublicSource, updatePublicSource, deletePublicSource, syncPublicSource } from '~/utils/services/public-source'
import { fetchGroups } from '~/utils/services/group'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardHeader from '~/components/ui/card/CardHeader.vue'
import CardTitle from '~/components/ui/card/CardTitle.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import CardFooter from '~/components/ui/card/CardFooter.vue'

const queryClient = useQueryClient()

const { data: sources, isLoading } = useQuery({
  queryKey: ['public-sources'],
  queryFn: () => fetchPublicSources(),
})

const { data: groups } = useQuery({
  queryKey: ['groups'],
  queryFn: () => fetchGroups(),
})

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const selectedSource = ref<PublicSource | null>(null)
const sourceUrl = ref('')
const sourceGroupId = ref('')
const isCreateSubmitting = ref(false)
const isEditSubmitting = ref(false)

const createMutation = useMutation({
  mutationFn: (data: CreatePublicSource) => createPublicSource(data),
  onSuccess: () => {
    showCreateDialog.value = false
    resetCreateForm()
    queryClient.invalidateQueries({ queryKey: ['public-sources'] })
  },
})

const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: string; data: UpdatePublicSource }) =>
    updatePublicSource(id, data),
  onSuccess: () => {
    showEditDialog.value = false
    resetEditForm()
    queryClient.invalidateQueries({ queryKey: ['public-sources'] })
  },
})

const deleteMutation = useMutation({
  mutationFn: (id: string) => deletePublicSource(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['public-sources'] })
  },
})

const syncMutation = useMutation({
  mutationFn: (id: string) => syncPublicSource(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['public-sources'] })
  },
})

function handleCreateSource() {
  if (!sourceUrl.value.trim() || !sourceGroupId.value || isCreateSubmitting.value) return
  isCreateSubmitting.value = true
  createMutation.mutate({
    url: sourceUrl.value,
    group_id: sourceGroupId.value,
  }, {
    onSettled: () => {
      isCreateSubmitting.value = false
    },
  })
}

function handleEditSource() {
  if (!selectedSource.value || !sourceUrl.value.trim() || !sourceGroupId.value || isEditSubmitting.value) return
  isEditSubmitting.value = true
  updateMutation.mutate({
    id: selectedSource.value.id,
    data: {
      url: sourceUrl.value,
      group_id: sourceGroupId.value,
    },
  }, {
    onSettled: () => {
      isEditSubmitting.value = false
    },
  })
}

function handleDeleteSource(source: PublicSource) {
  if (!confirm(`Are you sure you want to delete source "${source.url}"?`)) return
  deleteMutation.mutate(source.id)
}

function handleSyncSource(source: PublicSource) {
  syncMutation.mutate(source.id)
}

function openEditDialog(source: PublicSource) {
  updateMutation.reset()
  isEditSubmitting.value = false
  selectedSource.value = source
  sourceUrl.value = source.url
  sourceGroupId.value = source.group_id
  showEditDialog.value = true
}

function openCreateDialog() {
  createMutation.reset()
  resetCreateForm()
  showCreateDialog.value = true
}

function closeCreateDialog() {
  createMutation.reset()
  showCreateDialog.value = false
  resetCreateForm()
}

function closeEditDialog() {
  updateMutation.reset()
  showEditDialog.value = false
  resetEditForm()
}

function resetCreateForm() {
  sourceUrl.value = ''
  sourceGroupId.value = ''
  isCreateSubmitting.value = false
}

function resetEditForm() {
  sourceUrl.value = ''
  sourceGroupId.value = ''
  selectedSource.value = null
  isEditSubmitting.value = false
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex justify-end items-center">
      <UiButton @click="openCreateDialog">
        Add Source
      </UiButton>
    </div>

    <div v-if="isLoading" class="text-center text-muted-foreground py-8">
      Loading public sources...
    </div>
    <div v-else-if="!sources || sources.length === 0" class="text-center text-muted-foreground py-8">
      No public sources found
    </div>
    
    <UiCard v-for="source in sources" :key="source.id" class="p-4">
      <CardContent class="p-0">
        <div class="flex items-center justify-between">
          <div class="flex-1">
            <h3 class="font-semibold text-lg truncate">{{ source.url }}</h3>
            <p class="text-muted-foreground text-sm mt-1">
              Group: {{ groups?.find(g => g.id === source.group_id)?.name || source.group_id }}
            </p>
            <p class="text-muted-foreground text-sm">
              Last fetched: {{ source.last_fetched_at ? new Date(source.last_fetched_at).toLocaleString() : 'Never' }}
            </p>
          </div>
          <div class="flex gap-2">
            <UiButton
              variant="outline"
              size="sm"
              :disabled="syncMutation.isPending"
              @click="handleSyncSource(source)"
            >
              Sync
            </UiButton>
            <UiButton
              variant="outline"
              size="sm"
              @click="openEditDialog(source)"
            >
              Edit
            </UiButton>
            <UiButton
              variant="destructive"
              size="sm"
              :disabled="deleteMutation.isPending"
              @click="handleDeleteSource(source)"
            >
              Delete
            </UiButton>
          </div>
        </div>
      </CardContent>
    </UiCard>

    <UiCard v-if="showCreateDialog" class="mx-auto w-full max-w-md p-6">
        <CardHeader>
          <CardTitle>Add Public Source</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">URL</label>
            <UiInput
              v-model="sourceUrl"
              placeholder="https://example.com/subscription"
              @keyup.enter="handleCreateSource"
            />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Group</label>
            <select
              v-model="sourceGroupId"
              class="w-full px-3 py-2 border rounded-md bg-background"
            >
              <option value="">Select a group</option>
              <option v-for="group in groups" :key="group.id" :value="group.id">
                {{ group.name }}
              </option>
            </select>
          </div>
        </CardContent>
        <CardFooter class="flex justify-end gap-2">
          <UiButton variant="outline" @click="closeCreateDialog">
            Cancel
          </UiButton>
          <UiButton
            :disabled="!sourceUrl.trim() || !sourceGroupId || isCreateSubmitting"
            @click="handleCreateSource"
          >
            {{ isCreateSubmitting ? 'Adding...' : 'Add' }}
          </UiButton>
        </CardFooter>
    </UiCard>

    <UiCard v-if="showEditDialog" class="mx-auto w-full max-w-md p-6">
        <CardHeader>
          <CardTitle>Edit Public Source</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">URL</label>
            <UiInput
              v-model="sourceUrl"
              placeholder="https://example.com/subscription"
              @keyup.enter="handleEditSource"
            />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Group</label>
            <select
              v-model="sourceGroupId"
              class="w-full px-3 py-2 border rounded-md bg-background"
            >
              <option value="">Select a group</option>
              <option v-for="group in groups" :key="group.id" :value="group.id">
                {{ group.name }}
              </option>
            </select>
          </div>
        </CardContent>
        <CardFooter class="flex justify-end gap-2">
          <UiButton variant="outline" @click="closeEditDialog">
            Cancel
          </UiButton>
          <UiButton
            :disabled="!sourceUrl.trim() || !sourceGroupId || isEditSubmitting"
            @click="handleEditSource"
          >
            {{ isEditSubmitting ? 'Updating...' : 'Update' }}
          </UiButton>
        </CardFooter>
    </UiCard>
  </div>
</template>
