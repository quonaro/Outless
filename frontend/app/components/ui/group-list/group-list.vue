<script setup lang="ts">
import { ref, computed, unref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { Plus } from 'lucide-vue-next'
import type { Group, CreateGroup, UpdateGroup } from '~/utils/schemas/group'
import {
  fetchGroups,
  createGroup,
  updateGroup,
  deleteGroup,
  fetchTopUp,
  runTopUp,
  buildTopUpInput,
  defaultTopUpForm,
  type TopUpFormValues,
} from '~/utils/services/group'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiLabel from '~/components/ui/label/label.vue'
import UiCard from '~/components/ui/card/card.vue'
import TopUpFields from '~/components/TopUpFields.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'
import GroupsTopUpStatusDialog from '~/components/GroupsTopUpStatusDialog.vue'
import GroupActionsMenu from '~/components/GroupActionsMenu.vue'

const queryClient = useQueryClient()
const { confirm } = useConfirm()
const { info } = useToast()

const { data: groups, isLoading } = useQuery({
  queryKey: ['groups'],
  queryFn: () => fetchGroups(),
})

const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const selectedGroup = ref<Group | null>(null)
const groupName = ref('')
const randomEnabled = ref(false)
const randomLimit = ref<number | null>(null)
const showOrigins = ref(false)
const showTopUp = ref(false)
const topUpForm = ref<TopUpFormValues>(defaultTopUpForm())
const isCreateSubmitting = ref(false)
const isEditSubmitting = ref(false)
const isTopUpLoading = ref(false)
const isDeleting = computed(() => unref(deleteMutation.isPending) ?? false)
const runningTopUpId = ref<string | null>(null)

const createMutation = useMutation({
  mutationFn: (data: CreateGroup) => createGroup(data),
  onSuccess: () => {
    closeCreateDialog()
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})

const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: string; data: UpdateGroup }) => updateGroup(id, data),
  onSuccess: () => {
    closeEditDialog()
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})

const deleteMutation = useMutation({
  mutationFn: (id: string) => deleteGroup(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
  },
})

function buildPayload(): CreateGroup {
  const payload: CreateGroup = {
    name: groupName.value,
    random_enabled: randomEnabled.value,
    random_limit: randomLimit.value,
    show_origins: showOrigins.value,
  }
  if (showTopUp.value) {
    payload.top_up = buildTopUpInput(topUpForm.value)
  }
  return payload
}

function handleCreateGroup() {
  if (!groupName.value.trim() || isCreateSubmitting.value) return
  isCreateSubmitting.value = true
  createMutation.mutate(buildPayload(), {
    onSettled: () => {
      isCreateSubmitting.value = false
    },
  })
}

function handleEditGroup() {
  if (!selectedGroup.value || !groupName.value.trim() || isEditSubmitting.value) return
  isEditSubmitting.value = true
  const payload: UpdateGroup = buildPayload()
  updateMutation.mutate(
    { id: selectedGroup.value.id, data: payload },
    {
      onSettled: () => {
        isEditSubmitting.value = false
      },
    }
  )
}

async function handleDeleteGroup(group: Group) {
  const ok = await confirm({
    title: 'Delete group',
    message: `Are you sure you want to delete group "${group.name}"?`,
    variant: 'destructive',
  })
  if (!ok) return
  deleteMutation.mutate(group.id)
}

async function handleRunTopUp(group: Group) {
  if (!group.top_up_id) return
  runningTopUpId.value = group.id
  try {
    await runTopUp(group.top_up_id)
    info('Top-up run started', 'The group will be refilled in the background.')
  } finally {
    runningTopUpId.value = null
  }
}

function openEditDialog(group: Group) {
  updateMutation.reset()
  isEditSubmitting.value = false
  selectedGroup.value = group
  groupName.value = group.name
  randomEnabled.value = group.random_enabled ?? false
  randomLimit.value = group.random_limit ?? null
  showOrigins.value = group.show_origins ?? false
  showTopUp.value = group.is_topup ?? false
  topUpForm.value = defaultTopUpForm()
  if (group.is_topup && group.top_up_id) {
    loadTopUp(group.top_up_id)
  }
  showEditDialog.value = true
}

interface TopUpData {
  urls?: string[]
  parser_type?: string
  parser_params?: Record<string, unknown>
  check_enabled?: boolean
  check_config?: {
    workers?: number
    timeout?: string
    exclude_countries?: string[]
    max_latency?: string
    stages?: string[]
  }
  schedule_type?: string
  schedule_expr?: string
  enabled?: boolean
  next_run_at?: string
}

async function loadTopUp(id: string) {
  isTopUpLoading.value = true
  try {
    const data = await fetchTopUp(id)
    const response = data as { top_up?: TopUpData }
    const t = response.top_up
    if (!t) return
    topUpForm.value = {
      urlsText: (t.urls ?? []).join('\n'),
      parserType: t.parser_type || 'vless_lines',
      parserParams: t.parser_params || {},
      checkEnabled: t.check_enabled ?? false,
      workers: t.check_config?.workers ?? 2,
      timeout: t.check_config?.timeout ?? '5s',
      excludeCountries: (t.check_config?.exclude_countries ?? []).join(', '),
      maxLatency: t.check_config?.max_latency ?? '',
      stages: t.check_config?.stages ?? ['port', 'handshake'],
      scheduleType: t.schedule_type || 'interval',
      scheduleExpr: t.schedule_expr || '1h',
      enabled: t.enabled ?? true,
      nextRunAt: t.next_run_at ? t.next_run_at.replace('Z', '') : '',
    }
  } finally {
    isTopUpLoading.value = false
  }
}

function openCreateDialog() {
  createMutation.reset()
  isCreateSubmitting.value = false
  groupName.value = ''
  randomEnabled.value = false
  randomLimit.value = null
  showOrigins.value = false
  showTopUp.value = false
  topUpForm.value = defaultTopUpForm()
  showCreateDialog.value = true
}

function closeCreateDialog() {
  createMutation.reset()
  isCreateSubmitting.value = false
  showCreateDialog.value = false
}

function closeEditDialog() {
  updateMutation.reset()
  isEditSubmitting.value = false
  showEditDialog.value = false
  selectedGroup.value = null
  groupName.value = ''
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex justify-end items-center gap-2">
      <GroupsTopUpStatusDialog :groups="groups" />
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
            <div class="flex items-center gap-2">
              <h3 class="text-lg font-semibold">{{ group.name }}</h3>
              <span
                v-if="group.is_topup"
                class="inline-flex items-center rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
              >
                top-up
              </span>
            </div>
            <p class="mt-1 text-sm text-muted-foreground">ID: {{ group.id }}</p>
            <p class="text-sm text-muted-foreground">
              Created: {{ new Date(group.created_at).toLocaleString() }}
            </p>
          </div>
          <GroupActionsMenu
            :group="group"
            :is-deleting="isDeleting"
            :is-running="runningTopUpId === group.id"
            @edit="openEditDialog"
            @delete="handleDeleteGroup"
            @start-top-up="handleRunTopUp"
          />
        </div>
      </CardContent>
    </UiCard>

    <Sheet v-model:open="showCreateDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Create Group</SheetTitle>
          <SheetDescription>Create a new group of nodes.</SheetDescription>
        </SheetHeader>
        <div v-if="isTopUpLoading" class="py-8 text-center text-muted-foreground">Loading...</div>
        <div v-else class="grow min-h-0 space-y-4 overflow-y-auto py-4">
          <div class="space-y-2">
            <UiLabel>Group Name</UiLabel>
            <UiInput v-model="groupName" placeholder="Enter group name" />
          </div>

          <div class="flex items-center gap-2">
            <input id="create-show-origins" v-model="showOrigins" type="checkbox" class="h-4 w-4" />
            <UiLabel for="create-show-origins">Show direct node links</UiLabel>
          </div>

          <div class="flex items-center gap-2">
            <input id="create-top-up" v-model="showTopUp" type="checkbox" class="h-4 w-4" />
            <UiLabel for="create-top-up">Self-refilling (top-up) group</UiLabel>
          </div>

          <TopUpFields v-if="showTopUp" v-model="topUpForm" />
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
          <SheetDescription>Update group settings.</SheetDescription>
        </SheetHeader>
        <div v-if="isTopUpLoading" class="py-8 text-center text-muted-foreground">Loading...</div>
        <div v-else class="grow min-h-0 space-y-4 overflow-y-auto py-4">
          <div class="space-y-2">
            <UiLabel>Group Name</UiLabel>
            <UiInput v-model="groupName" placeholder="Enter group name" />
          </div>

          <div class="flex items-center gap-2">
            <input id="edit-show-origins" v-model="showOrigins" type="checkbox" class="h-4 w-4" />
            <UiLabel for="edit-show-origins">Show direct node links</UiLabel>
          </div>

          <div v-if="!selectedGroup?.is_topup" class="flex items-center gap-2">
            <input id="edit-top-up" v-model="showTopUp" type="checkbox" class="h-4 w-4" />
            <UiLabel for="edit-top-up">Self-refilling (top-up) group</UiLabel>
          </div>

          <TopUpFields v-if="showTopUp" v-model="topUpForm" />
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
