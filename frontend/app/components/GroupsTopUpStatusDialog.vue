<script setup lang="ts">
import { ref, computed } from 'vue'
import { Layers, Play } from 'lucide-vue-next'
import { useQueryClient } from '@tanstack/vue-query'
import type { Group } from '~/utils/schemas/group'
import { runAllTopUps, runTopUp, deleteGroup } from '~/utils/services/group'
import GroupActionsMenu from '~/components/GroupActionsMenu.vue'
import EditGroupDialog from '~/components/EditGroupDialog.vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import UiButton from '~/components/ui/button/button.vue'

const props = defineProps<{
  groups: Group[] | null | undefined
}>()

const queryClient = useQueryClient()
const { confirm } = useConfirm()
const { info, success, error } = useToast()

const showDialog = ref(false)
const isRunning = ref(false)
const deletingGroupId = ref<string | null>(null)
const runningTopUpId = ref<string | null>(null)
const showEditDialog = ref(false)
const selectedGroupForEdit = ref<Group | null>(null)

interface Result {
  status: 'none' | 'pending' | 'ok' | 'failed'
  total: number
  passed: number
  added: number
  error?: string
}

const results = ref<Record<string, Result>>({})

const topUpGroups = computed(() => (props.groups ?? []).filter((g) => g.is_topup))

const groupsWithResults = computed(() =>
  (props.groups ?? []).map((group) => {
    const existing = results.value[group.id]
    if (existing) return { group, result: existing }
    if (group.is_topup) {
      return { group, result: { status: 'pending' as const, total: 0, passed: 0, added: 0 } }
    }
    return { group, result: { status: 'none' as const, total: 0, passed: 0, added: 0 } }
  })
)

async function runAll() {
  isRunning.value = true
  try {
    const map: Record<string, Result> = {}
    for (const group of topUpGroups.value) {
      map[group.id] = { status: 'pending', total: 0, passed: 0, added: 0 }
    }
    results.value = map

    await runAllTopUps()
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    info('Top-up runs started', 'All top-up groups are being refilled in the background.')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    error('Top-up run failed', msg)
  } finally {
    isRunning.value = false
  }
}

async function handleStartTopUp(group: Group) {
  if (!group.top_up_id) return
  runningTopUpId.value = group.id
  try {
    await runTopUp(group.top_up_id)
    results.value[group.id] = { status: 'pending', total: 0, passed: 0, added: 0 }
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    info('Top-up run started', `Group "${group.name}" will be refilled in the background.`)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    error('Top-up failed', msg)
  } finally {
    runningTopUpId.value = null
  }
}

async function handleDelete(group: Group) {
  const ok = await confirm({
    title: 'Delete group',
    message: `Are you sure you want to delete group "${group.name}"?`,
    variant: 'destructive',
  })
  if (!ok) return

  deletingGroupId.value = group.id
  try {
    await deleteGroup(group.id)
    removeResult(group.id)
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    success('Group deleted', group.name)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    error('Delete failed', msg)
  } finally {
    deletingGroupId.value = null
  }
}

function removeResult(groupId: string) {
  const { [groupId]: _, ...rest } = results.value
  results.value = rest
}

function handleEdit(group: Group) {
  selectedGroupForEdit.value = group
  showEditDialog.value = true
}
</script>

<template>
  <Dialog v-model:open="showDialog">
    <UiButton variant="outline" @click="showDialog = true">
      <Layers class="h-4 w-4 mr-2" />
      Groups
    </UiButton>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Groups</DialogTitle>
        <DialogDescription>All groups and top-up status.</DialogDescription>
      </DialogHeader>
      <div class="max-h-[60vh] min-w-[20rem] overflow-y-auto py-4">
        <div class="mb-3 flex justify-end">
          <UiButton
            size="sm"
            variant="outline"
            :disabled="isRunning || topUpGroups.length === 0"
            @click="runAll"
          >
            <Play class="h-4 w-4 mr-2" />
            {{ isRunning ? 'Running...' : 'Run all top-ups' }}
          </UiButton>
        </div>
        <div v-if="!groups || groups.length === 0" class="py-8 text-center text-muted-foreground">
          No groups found.
        </div>
        <div v-else class="space-y-2">
          <div
            v-for="{ group, result } in groupsWithResults"
            :key="group.id"
            class="flex items-start justify-between rounded-md border p-3"
          >
            <div class="min-w-0">
              <p class="font-medium truncate">{{ group.name || group.id }}</p>
              <p class="text-xs text-muted-foreground">
                <span
                  v-if="group.is_topup"
                  class="inline-flex items-center rounded-full bg-primary/10 px-1.5 py-0.5 text-xs font-medium text-primary"
                >
                  top-up
                </span>
                <span v-else>regular</span>
              </p>
              <p
                v-if="result.status === 'ok' || result.status === 'failed'"
                class="text-xs text-muted-foreground"
              >
                Total: {{ result.total }} · Passed: {{ result.passed }} · Added: {{ result.added }}
              </p>
              <p v-if="result.error" class="mt-1 text-xs text-red-500">
                {{ result.error }}
              </p>
            </div>
            <span
              v-if="result.status === 'ok' || result.status === 'failed'"
              :class="[
                'shrink-0 rounded-full px-2 py-0.5 text-xs font-medium',
                result.status === 'ok'
                  ? 'bg-emerald-100 text-emerald-700'
                  : 'bg-red-100 text-red-700',
              ]"
            >
              {{ result.status }}
            </span>
            <span
              v-else-if="result.status === 'pending'"
              class="shrink-0 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground"
            >
              pending
            </span>
            <GroupActionsMenu
              :group="group"
              :is-deleting="deletingGroupId === group.id"
              :is-running="runningTopUpId === group.id"
              @edit="handleEdit"
              @delete="handleDelete"
              @start-top-up="handleStartTopUp"
            />
          </div>
        </div>
        <EditGroupDialog v-model:open="showEditDialog" :group="selectedGroupForEdit" />
      </div>
    </DialogContent>
  </Dialog>
</template>
