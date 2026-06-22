<script setup lang="ts">
/* eslint-disable max-lines */
import { computed, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { useQueryClient } from '@tanstack/vue-query'
import type { CreateToken, IssuedToken, Token } from '~/utils/schemas/token'
import { EXPIRES_IN_OPTIONS } from '~/utils/schemas/token'
import { useTokens } from '~/composables/tokens/useTokens'
import { useCreateToken } from '~/composables/tokens/useCreateToken'
import { useDeleteToken } from '~/composables/tokens/useDeleteToken'
import { useRemoveToken } from '~/composables/tokens/useRemoveToken'
import { useActivateToken } from '~/composables/tokens/useActivateToken'
import { useUpdateToken } from '~/composables/tokens/useUpdateToken'
import { useGroups } from '~/composables/groups/useGroups'
import { useInbounds } from '~/composables/inbounds/useInbounds'
import { Plus, MoreVertical, Eye, Pencil, Power, PowerOff, Trash2, Copy } from 'lucide-vue-next'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'

const config = useRuntimeConfig()
const apiBase = (config.public.apiBase as string).replace(/\/+$/, '')
const queryClient = useQueryClient()
const defaultExpiresIn = EXPIRES_IN_OPTIONS[2]?.value ?? '720h'

const { data: tokens, isLoading } = useTokens()
const { data: groups } = useGroups()
const { data: inbounds } = useInbounds()
const { confirm } = useConfirm()

const showCreateDialog = ref(false)
const showIssuedDialog = ref(false)
const showAccessURLDialog = ref(false)
const showEditDialog = ref(false)
const issuedToken = ref<IssuedToken | null>(null)
const selectedAccessURL = ref('')
const issuedAccessURL = ref('')
const isIssueSubmitting = ref(false)
const pendingDeactivateId = ref('')
const pendingActivateId = ref('')
const pendingRemoveId = ref('')
const editingTokenId = ref('')
const isEditSubmitting = ref(false)

const ownerInput = ref('')
const groupIdsInput = ref<string[]>([])
const inboundIdsInput = ref<string[]>([])
const expiresInInput = ref(defaultExpiresIn)
const quotaBytesInput = ref<number | undefined>(undefined)
const quotaPeriodInput = ref('')

const editOwnerInput = ref('')
const editGroupIdsInput = ref<string[]>([])
const editInboundIdsInput = ref<string[]>([])
const editExpiresInInput = ref(defaultExpiresIn)
const editQuotaBytesInput = ref<number | undefined>(undefined)
const editQuotaPeriodInput = ref('')

const issuedUrlRef = ref<HTMLPreElement>()
const selectedUrlRef = ref<HTMLPreElement>()

function useAutoScroll(elRef: typeof issuedUrlRef) {
  let rafId: number | null = null

  function start() {
    const el = elRef.value
    if (!el) return
    const maxScroll = el.scrollWidth - el.clientWidth
    if (maxScroll <= 0) return

    let direction = 1
    let pos = 0
    const speed = 0.2

    function step() {
      if (!el) return
      pos += direction * speed
      if (pos >= maxScroll) {
        pos = maxScroll
        direction = -1
      } else if (pos <= 0) {
        pos = 0
        direction = 1
      }
      el.scrollLeft = pos
      rafId = requestAnimationFrame(step)
    }

    rafId = requestAnimationFrame(step)
  }

  function stop() {
    if (rafId !== null) {
      cancelAnimationFrame(rafId)
      rafId = null
    }
    const el = elRef.value
    if (el) el.scrollLeft = 0
  }

  return { start, stop }
}

const issuedScroll = useAutoScroll(issuedUrlRef)
const selectedScroll = useAutoScroll(selectedUrlRef)

watch(showIssuedDialog, (open) => {
  if (open) {
    setTimeout(() => issuedScroll.start(), 100)
  } else {
    issuedScroll.stop()
  }
})

watch(showAccessURLDialog, (open) => {
  if (open) {
    setTimeout(() => selectedScroll.start(), 100)
  } else {
    selectedScroll.stop()
  }
})

const invalidate = () => queryClient.invalidateQueries({ queryKey: ['tokens'] })

const createMutation = useCreateToken({
  onSuccess: (token) => {
    issuedToken.value = token
    issuedAccessURL.value = resolveAccessURL(token)
    showIssuedDialog.value = true
    showCreateDialog.value = false
    resetForm()
    invalidate()
  },
})

const deleteMutation = useDeleteToken({
  onSuccess: () => {
    toast.success('Token deactivated successfully')
    invalidate()
  },
  onError: (err) => {
    toast.error('Failed to deactivate token', {
      description: err.message,
    })
  },
})
const activateMutation = useActivateToken({
  onSuccess: () => {
    toast.success('Token activated successfully')
    invalidate()
  },
  onError: (err) => {
    toast.error('Failed to activate token', {
      description: err.message,
    })
  },
})
const removeMutation = useRemoveToken({
  onSuccess: () => {
    toast.success('Token removed successfully')
    invalidate()
  },
  onError: (err) => {
    toast.error('Failed to remove token', {
      description: err.message,
    })
  },
})
const updateMutation = useUpdateToken({
  onSuccess: () => {
    toast.success('Token updated successfully')
    invalidate()
  },
  onError: (err) => {
    toast.error('Failed to update token', {
      description: err.message,
    })
  },
})

const groupNameById = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  for (const group of groups.value ?? []) {
    map[group.id] = group.name
  }
  return map
})

const inboundNameById = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  for (const inbound of inbounds.value ?? []) {
    map[inbound.id] = inbound.name
  }
  return map
})

const sortedTokens = computed<Token[]>(() => {
  const list = tokens.value ?? []
  return [...list].sort((a, b) => b.created_at.localeCompare(a.created_at))
})

function resetForm() {
  ownerInput.value = ''
  groupIdsInput.value = []
  inboundIdsInput.value = []
  expiresInInput.value = defaultExpiresIn
  quotaBytesInput.value = undefined
  quotaPeriodInput.value = ''
  isIssueSubmitting.value = false
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

function closeIssuedDialog() {
  showIssuedDialog.value = false
  issuedToken.value = null
  issuedAccessURL.value = ''
}

function handleCreate() {
  if (!ownerInput.value.trim() || !expiresInInput.value || isIssueSubmitting.value) return
  const payload: CreateToken = {
    owner: ownerInput.value.trim(),
    group_ids: groupIdsInput.value,
    inbound_ids: inboundIdsInput.value,
    expires_in: expiresInInput.value,
    quota_bytes: quotaBytesInput.value,
    quota_period: quotaPeriodInput.value,
  }
  isIssueSubmitting.value = true
  createMutation.mutate(payload, {
    onSettled: () => {
      isIssueSubmitting.value = false
    },
  })
}

async function handleDeactivate(token: Token) {
  const ok = await confirm({
    title: 'Deactivate token',
    message: `Deactivate token for ${token.owner}?`,
    variant: 'destructive',
  })
  if (!ok) return
  pendingDeactivateId.value = token.id
  deleteMutation.mutate(token.id, {
    onSettled: () => {
      pendingDeactivateId.value = ''
    },
  })
}

async function handleActivate(token: Token) {
  const ok = await confirm({
    title: 'Activate token',
    message: `Activate token for ${token.owner}?`,
  })
  if (!ok) return
  pendingActivateId.value = token.id
  activateMutation.mutate(token.id, {
    onSettled: () => {
      pendingActivateId.value = ''
    },
  })
}

async function handleRemove(token: Token) {
  const ok = await confirm({
    title: 'Remove token',
    message: `Remove token for ${token.owner}? This action cannot be undone.`,
    variant: 'destructive',
  })
  if (!ok) return
  pendingRemoveId.value = token.id
  removeMutation.mutate(token.id, {
    onSettled: () => {
      pendingRemoveId.value = ''
    },
  })
}

function resolveAccessURL(token: Pick<Token, 'access_url'>): string {
  if (!token.access_url) return ''
  if (token.access_url.startsWith('http://') || token.access_url.startsWith('https://')) {
    return token.access_url
  }
  const path = token.access_url.startsWith('/') ? token.access_url : `/${token.access_url}`
  let base = apiBase
  if (base.startsWith('/')) {
    const origin = typeof window !== 'undefined' ? window.location.origin : ''
    base = origin + base
  }
  return `${base}${path}`
}

function formatBytes(v: number): string {
  if (v === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.max(0, Math.floor(Math.log10(v) / 3))
  const unit = units[Math.min(i, units.length - 1)]
  const scaled = v / Math.pow(1000, Math.min(i, units.length - 1))
  return `${scaled.toFixed(2)} ${unit}`
}

async function copyText(value: string) {
  if (!value) return
  try {
    await navigator.clipboard?.writeText(value)
  } catch {
    // clipboard may be unavailable outside secure context
  }
}

function viewAccessURL(token: Token) {
  selectedAccessURL.value = resolveAccessURL(token)
  if (!selectedAccessURL.value) {
    toast.error('Access URL is unavailable for this legacy token. Re-issue token to recover URL.')
    return
  }
  showAccessURLDialog.value = true
}

async function copyAccessURL(token: Token) {
  const url = resolveAccessURL(token)
  if (!url) {
    toast.error('Access URL is unavailable for this legacy token. Re-issue token to recover URL.')
    return
  }
  try {
    await navigator.clipboard?.writeText(url)
    toast.success('URL copied to clipboard')
  } catch {
    toast.error('Failed to copy URL')
  }
}

function statusBadge(token: Token): { label: string; cls: string } {
  if (!token.is_active) {
    return { label: 'inactive', cls: 'bg-muted text-muted-foreground' }
  }
  const expired = new Date(token.expires_at).getTime() <= Date.now()
  if (expired) {
    return {
      label: 'expired',
      cls: 'bg-red-500/20 text-red-600 dark:text-red-400',
    }
  }
  return {
    label: 'active',
    cls: 'bg-emerald-500/20 text-emerald-600 dark:text-emerald-400',
  }
}

function tokenGroupLabels(token: Token): string {
  const groupIDs = token.group_ids?.length
    ? token.group_ids
    : token.group_id
      ? [token.group_id]
      : []
  if (groupIDs.length === 0) return 'All groups'
  return groupIDs.map((id) => groupNameById.value[id] ?? id).join(', ')
}

function tokenInboundLabels(token: Token): string {
  const inboundIDs = token.inbound_ids ?? []
  if (inboundIDs.length === 0) return 'All inbounds'
  return inboundIDs.map((id) => inboundNameById.value[id] ?? id).join(', ')
}

function toggleGroupSelection(groupID: string, checked: boolean) {
  if (checked) {
    groupIdsInput.value = [...groupIdsInput.value.filter((id) => id !== groupID), groupID]
    return
  }
  groupIdsInput.value = groupIdsInput.value.filter((id) => id !== groupID)
}

function handleGroupCheckboxChange(groupID: string, event: Event) {
  const target = event.target as HTMLInputElement | null
  if (!target) return
  toggleGroupSelection(groupID, target.checked)
}

function toggleInboundSelection(inboundID: string, checked: boolean) {
  if (checked) {
    inboundIdsInput.value = [...inboundIdsInput.value.filter((id) => id !== inboundID), inboundID]
    return
  }
  inboundIdsInput.value = inboundIdsInput.value.filter((id) => id !== inboundID)
}

function handleInboundCheckboxChange(inboundID: string, event: Event) {
  const target = event.target as HTMLInputElement | null
  if (!target) return
  toggleInboundSelection(inboundID, target.checked)
}

function toggleEditInboundSelection(inboundID: string, checked: boolean) {
  if (checked) {
    editInboundIdsInput.value = [
      ...editInboundIdsInput.value.filter((id) => id !== inboundID),
      inboundID,
    ]
    return
  }
  editInboundIdsInput.value = editInboundIdsInput.value.filter((id) => id !== inboundID)
}

function handleEditInboundCheckboxChange(inboundID: string, event: Event) {
  const target = event.target as HTMLInputElement | null
  if (!target) return
  toggleEditInboundSelection(inboundID, target.checked)
}

function openEditDialog(token: Token) {
  updateMutation.reset()
  editingTokenId.value = token.id
  editOwnerInput.value = token.owner
  editGroupIdsInput.value = token.group_ids?.length
    ? token.group_ids
    : token.group_id
      ? [token.group_id]
      : []
  editInboundIdsInput.value = token.inbound_ids ?? []
  editExpiresInInput.value = defaultExpiresIn
  editQuotaBytesInput.value = token.quota_bytes ?? undefined
  editQuotaPeriodInput.value = token.quota_period ?? ''
  isEditSubmitting.value = false
  showEditDialog.value = true
}

function closeEditDialog() {
  updateMutation.reset()
  showEditDialog.value = false
  editingTokenId.value = ''
  editOwnerInput.value = ''
  editGroupIdsInput.value = []
  editInboundIdsInput.value = []
  editExpiresInInput.value = defaultExpiresIn
  editQuotaBytesInput.value = undefined
  editQuotaPeriodInput.value = ''
  isEditSubmitting.value = false
}

function handleEdit() {
  if (
    !editOwnerInput.value.trim() ||
    !editExpiresInInput.value ||
    isEditSubmitting.value ||
    !editingTokenId.value
  )
    return
  const payload = {
    owner: editOwnerInput.value.trim(),
    group_ids: editGroupIdsInput.value,
    inbound_ids: editInboundIdsInput.value,
    expires_in: editExpiresInInput.value,
    quota_bytes: editQuotaBytesInput.value,
    quota_period: editQuotaPeriodInput.value,
  }
  isEditSubmitting.value = true
  updateMutation.mutate(
    { id: editingTokenId.value, token: payload },
    {
      onSettled: () => {
        isEditSubmitting.value = false
        closeEditDialog()
      },
    }
  )
}

function toggleEditGroupSelection(groupID: string, checked: boolean) {
  if (checked) {
    editGroupIdsInput.value = [...editGroupIdsInput.value.filter((id) => id !== groupID), groupID]
    return
  }
  editGroupIdsInput.value = editGroupIdsInput.value.filter((id) => id !== groupID)
}

function handleEditGroupCheckboxChange(groupID: string, event: Event) {
  const target = event.target as HTMLInputElement | null
  if (!target) return
  toggleEditGroupSelection(groupID, target.checked)
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-start gap-3">
      <UiButton @click="openCreateDialog">
        <Plus class="h-4 w-4" />
        Issue Token
      </UiButton>
    </div>

    <div v-if="isLoading" class="py-8 text-center text-muted-foreground">Loading tokens...</div>
    <div v-else-if="sortedTokens.length === 0" class="py-8 text-center text-muted-foreground">
      No tokens issued yet
    </div>

    <UiCard v-for="token in sortedTokens" :key="token.id" class="p-4">
      <CardContent class="p-0">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium uppercase"
                :class="statusBadge(token).cls"
              >
                {{ statusBadge(token).label }}
              </span>
              <span class="truncate text-base font-semibold">{{ token.owner }}</span>
            </div>
            <p class="text-sm text-muted-foreground">
              Groups:
              <span class="font-medium">{{ tokenGroupLabels(token) }}</span>
            </p>
            <p class="text-sm text-muted-foreground">
              Inbounds:
              <span class="font-medium">{{ tokenInboundLabels(token) }}</span>
            </p>
            <p v-if="token.quota_bytes && token.quota_period" class="text-sm text-muted-foreground">
              Quota: {{ formatBytes(token.quota_bytes) }} / {{ token.quota_period }}
            </p>
            <p class="text-sm text-muted-foreground">
              Expires: {{ new Date(token.expires_at).toLocaleString() }} · Created:
              {{ new Date(token.created_at).toLocaleString() }}
            </p>
          </div>
          <div class="flex shrink-0 gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <UiButton variant="ghost" size="icon">
                  <MoreVertical class="h-4 w-4" />
                </UiButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem @click="viewAccessURL(token)">
                  <Eye class="h-4 w-4 mr-2" />
                  View URL
                </DropdownMenuItem>
                <DropdownMenuItem @click="copyAccessURL(token)">
                  <Copy class="h-4 w-4 mr-2" />
                  Copy URL
                </DropdownMenuItem>
                <DropdownMenuItem @click="openEditDialog(token)">
                  <Pencil class="h-4 w-4 mr-2" />
                  Edit
                </DropdownMenuItem>
                <template v-if="token.is_active">
                  <DropdownMenuItem
                    class="text-destructive focus:text-destructive"
                    :disabled="pendingDeactivateId === token.id"
                    @click="handleDeactivate(token)"
                  >
                    <PowerOff class="h-4 w-4 mr-2" />
                    Deactivate
                  </DropdownMenuItem>
                </template>
                <template v-else>
                  <DropdownMenuItem
                    :disabled="pendingActivateId === token.id"
                    @click="handleActivate(token)"
                  >
                    <Power class="h-4 w-4 mr-2" />
                    Activate
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    class="text-destructive focus:text-destructive"
                    :disabled="pendingRemoveId === token.id"
                    @click="handleRemove(token)"
                  >
                    <Trash2 class="h-4 w-4 mr-2" />
                    Remove
                  </DropdownMenuItem>
                </template>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </CardContent>
    </UiCard>

    <Sheet v-model:open="showCreateDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Issue Token</SheetTitle>
          <SheetDescription>Create a new access token for a user or device.</SheetDescription>
        </SheetHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">Owner</label>
            <UiInput
              v-model="ownerInput"
              placeholder="user@example.com"
              @keyup.enter="handleCreate"
            />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Group Access</label>
            <div class="max-h-40 space-y-2 overflow-auto rounded-md border p-2">
              <label class="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  :checked="groupIdsInput.length === 0"
                  @change="groupIdsInput = []"
                />
                <span>All groups</span>
              </label>
              <label v-for="g in groups ?? []" :key="g.id" class="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  :checked="groupIdsInput.includes(g.id)"
                  @change="handleGroupCheckboxChange(g.id, $event)"
                />
                <span>{{ g.name }}</span>
              </label>
            </div>
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Inbound Access</label>
            <div class="max-h-40 space-y-2 overflow-auto rounded-md border p-2">
              <label class="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  :checked="inboundIdsInput.length === 0"
                  @change="inboundIdsInput = []"
                />
                <span>All inbounds</span>
              </label>
              <label
                v-for="ib in inbounds ?? []"
                :key="ib.id"
                class="flex items-center gap-2 text-sm"
              >
                <input
                  type="checkbox"
                  :checked="inboundIdsInput.includes(ib.id)"
                  @change="handleInboundCheckboxChange(ib.id, $event)"
                />
                <span>{{ ib.name }}</span>
              </label>
            </div>
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Expires in</label>
            <select
              v-model="expiresInInput"
              class="w-full rounded-md border bg-background px-3 py-2 text-sm"
            >
              <option v-for="opt in EXPIRES_IN_OPTIONS" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-medium">Quota (bytes)</label>
              <UiInput
                v-model.number="quotaBytesInput"
                type="number"
                placeholder="e.g. 1073741824"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Quota Period</label>
              <select
                v-model="quotaPeriodInput"
                class="w-full rounded-md border bg-background px-3 py-2 text-sm"
              >
                <option value="">None</option>
                <option value="day">Day</option>
                <option value="month">Month</option>
              </select>
            </div>
          </div>
        </div>
        <SheetFooter>
          <UiButton variant="outline" @click="closeCreateDialog"> Cancel </UiButton>
          <UiButton :disabled="!ownerInput.trim() || isIssueSubmitting" @click="handleCreate">
            {{ isIssueSubmitting ? 'Issuing...' : 'Issue' }}
          </UiButton>
        </SheetFooter>
      </SheetContent>
    </Sheet>

    <Sheet v-model:open="showEditDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Edit Token</SheetTitle>
          <SheetDescription>Update token access and expiration.</SheetDescription>
        </SheetHeader>
        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <label class="text-sm font-medium">Owner</label>
            <UiInput
              v-model="editOwnerInput"
              placeholder="user@example.com"
              @keyup.enter="handleEdit"
            />
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Group Access</label>
            <div class="max-h-40 space-y-2 overflow-auto rounded-md border p-2">
              <label class="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  :checked="editGroupIdsInput.length === 0"
                  @change="editGroupIdsInput = []"
                />
                <span>All groups</span>
              </label>
              <label v-for="g in groups ?? []" :key="g.id" class="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  :checked="editGroupIdsInput.includes(g.id)"
                  @change="handleEditGroupCheckboxChange(g.id, $event)"
                />
                <span>{{ g.name }}</span>
              </label>
            </div>
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Inbound Access</label>
            <div class="max-h-40 space-y-2 overflow-auto rounded-md border p-2">
              <label class="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  :checked="editInboundIdsInput.length === 0"
                  @change="editInboundIdsInput = []"
                />
                <span>All inbounds</span>
              </label>
              <label
                v-for="ib in inbounds ?? []"
                :key="ib.id"
                class="flex items-center gap-2 text-sm"
              >
                <input
                  type="checkbox"
                  :checked="editInboundIdsInput.includes(ib.id)"
                  @change="handleEditInboundCheckboxChange(ib.id, $event)"
                />
                <span>{{ ib.name }}</span>
              </label>
            </div>
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Expires in</label>
            <select
              v-model="editExpiresInInput"
              class="w-full rounded-md border bg-background px-3 py-2 text-sm"
            >
              <option v-for="opt in EXPIRES_IN_OPTIONS" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-medium">Quota (bytes)</label>
              <UiInput
                v-model.number="editQuotaBytesInput"
                type="number"
                placeholder="e.g. 1073741824"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Quota Period</label>
              <select
                v-model="editQuotaPeriodInput"
                class="w-full rounded-md border bg-background px-3 py-2 text-sm"
              >
                <option value="">None</option>
                <option value="day">Day</option>
                <option value="month">Month</option>
              </select>
            </div>
          </div>
        </div>
        <SheetFooter>
          <UiButton variant="outline" @click="closeEditDialog"> Cancel </UiButton>
          <UiButton :disabled="!editOwnerInput.trim() || isEditSubmitting" @click="handleEdit">
            {{ isEditSubmitting ? 'Updating...' : 'Update' }}
          </UiButton>
        </SheetFooter>
      </SheetContent>
    </Sheet>

    <Sheet v-model:open="showIssuedDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Token Issued</SheetTitle>
          <SheetDescription>Token has been generated successfully.</SheetDescription>
        </SheetHeader>
        <div v-if="issuedToken" class="space-y-3 py-4">
          <p class="text-sm text-muted-foreground">
            Copy and share this subscription URL. Users can import it directly in VLESS clients.
          </p>
          <pre
            ref="issuedUrlRef"
            class="overflow-x-auto whitespace-nowrap rounded-md border bg-muted/40 p-3 font-mono text-xs no-scrollbar"
            >{{ issuedAccessURL }}</pre
          >
        </div>
        <SheetFooter>
          <UiButton variant="outline" @click="copyText(issuedAccessURL)">Copy URL</UiButton>
          <UiButton @click="closeIssuedDialog">Close</UiButton>
        </SheetFooter>
      </SheetContent>
    </Sheet>

    <Sheet v-model:open="showAccessURLDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Access URL</SheetTitle>
          <SheetDescription>Subscription URL for the selected token.</SheetDescription>
        </SheetHeader>
        <div v-if="selectedAccessURL" class="py-4">
          <pre
            ref="selectedUrlRef"
            class="overflow-x-auto whitespace-nowrap rounded-md border bg-muted/40 p-3 font-mono text-xs no-scrollbar"
            >{{ selectedAccessURL }}</pre
          >
        </div>
        <SheetFooter>
          <UiButton variant="outline" @click="copyText(selectedAccessURL)">Copy</UiButton>
          <UiButton @click="showAccessURLDialog = false">Close</UiButton>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  </div>
</template>

<style scoped>
.no-scrollbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.no-scrollbar::-webkit-scrollbar {
  display: none;
}
</style>
