<script setup lang="ts">
import { ref, computed } from 'vue'
import { Layers, Play } from 'lucide-vue-next'
import { useQueryClient } from '@tanstack/vue-query'
import type { Group } from '~/utils/schemas/group'
import { runAllTopUps, runTopUp, deleteGroup } from '~/utils/services/group'
import { useTopUpProgress, type TopUpProgress } from '~/composables/groups/useTopUpProgress'
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
const { progress: topUpProgress } = useTopUpProgress()

const showDialog = ref(false)
const isRunning = ref(false)
const deletingGroupId = ref<string | null>(null)
const runningTopUpId = ref<string | null>(null)
const showEditDialog = ref(false)
const selectedGroupForEdit = ref<Group | null>(null)

const topUpGroups = computed(() => (props.groups ?? []).filter((g) => g.is_topup))
const anyRunning = computed(() =>
  topUpGroups.value.some((g) => topUpProgress.value[g.id]?.status === 'running')
)
const isRunAllDisabled = computed(
  () => isRunning.value || anyRunning.value || topUpGroups.value.length === 0
)

function groupProgress(group: Group): TopUpProgress {
  return (
    topUpProgress.value[group.id] ?? {
      top_up_id: group.top_up_id,
      group_id: group.id,
      group_name: group.name,
      status: 'pending',
      stage: 'idle',
      total: 0,
      checked: 0,
      passed: 0,
      added: 0,
      failed: 0,
    }
  )
}

function groupStatus(group: Group): string {
  if (!group.is_topup) return 'none'
  return groupProgress(group).status
}

function statusClasses(status: string): string {
  switch (status) {
    case 'running':
      return 'bg-accent/10 text-accent'
    case 'completed':
      return 'bg-primary/10 text-primary'
    case 'failed':
      return 'bg-destructive/10 text-destructive'
    case 'pending':
      return 'bg-muted text-muted-foreground'
    default:
      return 'bg-muted/50 text-muted-foreground'
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'running':
      return 'running'
    case 'completed':
      return 'completed'
    case 'failed':
      return 'failed'
    case 'pending':
      return 'pending'
    default:
      return 'idle'
  }
}

function progressPercent(p: TopUpProgress): number {
  if (p.total <= 0) return 0
  return Math.min(100, Math.round((p.checked / p.total) * 100))
}

function truncateCurrentUrl(url?: string, max = 64): string {
  if (!url) return ''
  if (url.length <= max) return url
  return url.slice(0, max - 3) + '...'
}

async function runAll() {
  if (isRunAllDisabled.value) return
  isRunning.value = true
  try {
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
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    success('Group deleted', group.name)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    error('Delete failed', msg)
  } finally {
    deletingGroupId.value = null
  }
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
    <DialogContent class="max-w-2xl">
      <DialogHeader>
        <DialogTitle>Groups</DialogTitle>
        <DialogDescription>All groups and live top-up progress.</DialogDescription>
      </DialogHeader>
      <div class="max-h-[70vh] min-w-[20rem] overflow-y-auto py-4">
        <div class="mb-4 flex items-center justify-between gap-3">
          <p class="text-sm text-muted-foreground">
            {{ topUpGroups.length }} top-up group{{ topUpGroups.length === 1 ? '' : 's' }}
          </p>
          <UiButton size="sm" variant="outline" :disabled="isRunAllDisabled" @click="runAll">
            <Play class="h-4 w-4 mr-2" />
            {{
              isRunAllDisabled ? (anyRunning ? 'Running...' : 'Run all top-ups') : 'Run all top-ups'
            }}
          </UiButton>
        </div>
        <div v-if="!groups || groups.length === 0" class="py-8 text-center text-muted-foreground">
          No groups found.
        </div>
        <div v-else class="space-y-3">
          <div v-for="group in groups" :key="group.id" class="rounded-lg border p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <p class="font-medium truncate">{{ group.name || group.id }}</p>
                  <span class="text-xs text-muted-foreground">({{ group.total_nodes }} nodes)</span>
                </div>
                <div class="mt-1 flex flex-wrap items-center gap-2">
                  <span
                    v-if="group.is_topup"
                    class="inline-flex items-center rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
                  >
                    top-up
                  </span>
                  <span v-else class="text-xs text-muted-foreground">regular</span>
                  <span
                    :class="[
                      'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                      statusClasses(groupStatus(group)),
                    ]"
                  >
                    {{ statusLabel(groupStatus(group)) }}
                  </span>
                </div>
              </div>
              <GroupActionsMenu
                :group="group"
                :is-deleting="deletingGroupId === group.id"
                :is-running="
                  groupProgress(group)?.status === 'running' || runningTopUpId === group.id
                "
                @edit="handleEdit"
                @delete="handleDelete"
                @start-top-up="handleStartTopUp"
              />
            </div>

            <div v-if="group.is_topup && groupProgress(group)" class="mt-4 space-y-3">
              <div class="space-y-1">
                <div class="flex items-center justify-between text-xs">
                  <span class="font-medium uppercase tracking-wide text-muted-foreground">{{
                    groupProgress(group).stage
                  }}</span>
                  <span class="text-muted-foreground"
                    >{{ groupProgress(group).checked }}/{{ groupProgress(group).total }}</span
                  >
                </div>
                <div class="h-2 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    class="h-full rounded-full bg-primary transition-all duration-300"
                    :style="{ width: `${progressPercent(groupProgress(group))}%` }"
                  />
                </div>
              </div>

              <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
                <div class="rounded-md bg-muted/50 p-2 text-center">
                  <p class="text-xs text-muted-foreground">Total</p>
                  <p class="text-sm font-semibold">{{ groupProgress(group).total }}</p>
                </div>
                <div class="rounded-md bg-primary/10 p-2 text-center">
                  <p class="text-xs text-primary">Passed</p>
                  <p class="text-sm font-semibold text-primary">
                    {{ groupProgress(group).passed }}
                  </p>
                </div>
                <div class="rounded-md bg-destructive/10 p-2 text-center">
                  <p class="text-xs text-destructive">Failed</p>
                  <p class="text-sm font-semibold text-destructive">
                    {{ groupProgress(group).failed }}
                  </p>
                </div>
                <div class="rounded-md bg-secondary/10 p-2 text-center">
                  <p class="text-xs text-secondary">Imported</p>
                  <p class="text-sm font-semibold text-secondary">
                    {{ groupProgress(group).added }}
                  </p>
                </div>
              </div>

              <div v-if="groupProgress(group).current_url" class="rounded-md bg-muted/50 p-2">
                <p class="mb-1 text-xs text-muted-foreground">Current VLESS</p>
                <p class="break-all font-mono text-xs" :title="groupProgress(group).current_url">
                  {{ truncateCurrentUrl(groupProgress(group).current_url) }}
                </p>
              </div>

              <p v-if="groupProgress(group).error" class="text-xs text-destructive">
                {{ groupProgress(group).error }}
              </p>
            </div>
          </div>
        </div>
        <EditGroupDialog v-model:open="showEditDialog" :group="selectedGroupForEdit" />
      </div>
    </DialogContent>
  </Dialog>
</template>
