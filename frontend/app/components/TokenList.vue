<script setup lang="ts">
/* eslint-disable max-lines */
import { computed, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { useQueryClient } from '@tanstack/vue-query'
import type { CreateToken, IssuedToken, Token } from '~/utils/schemas/token'
import { EXPIRES_IN_OPTIONS } from '~/utils/schemas/token'
import { formatBytes, parseByteSize, bytesToSize, periodLabel } from '~/utils/bytes'
import { useTokens } from '~/composables/tokens/useTokens'
import { useCreateToken } from '~/composables/tokens/useCreateToken'
import { useDeleteToken } from '~/composables/tokens/useDeleteToken'
import { useRemoveToken } from '~/composables/tokens/useRemoveToken'
import { useActivateToken } from '~/composables/tokens/useActivateToken'
import { useUpdateToken } from '~/composables/tokens/useUpdateToken'
import { useResetTrafficToken } from '~/composables/tokens/useResetTrafficToken'
import { useTokenTraffic } from '~/composables/tokens/useTokenTraffic'
import { useGroups } from '~/composables/groups/useGroups'
import { useInbounds } from '~/composables/inbounds/useInbounds'
import { batchDeactivateTokens, batchRemoveTokens } from '~/utils/services/token'
import {
  Plus,
  MoreVertical,
  Eye,
  Pencil,
  Power,
  PowerOff,
  Trash2,
  Copy,
  RotateCcw,
  BarChart3,
  Globe,
  ArrowLeftRight,
  Wifi,
  Activity,
  Calendar,
  Shield,
} from 'lucide-vue-next'
import TokenTrafficChart from '~/components/TokenTrafficChart.vue'
import TokenIPRestrictions from '~/components/TokenIPRestrictions.vue'
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
const showTrafficDialog = ref(false)
const trafficTokenId = ref('')
const showIPRestrictionsDialog = ref(false)
const ipRestrictionTokenId = ref('')
const issuedToken = ref<IssuedToken | null>(null)
const selectedAccessURL = ref('')
const issuedAccessURL = ref('')
const isIssueSubmitting = ref(false)
const pendingDeactivateId = ref('')
const pendingActivateId = ref('')
const pendingRemoveId = ref('')
const pendingResetTrafficId = ref('')
const editingTokenId = ref('')
const isEditSubmitting = ref(false)
const selectedTokenIDs = ref<Set<string>>(new Set())

const ownerInput = ref('')
const groupIdsInput = ref<string[]>([])
const inboundIdsInput = ref<string[]>([])
const expiresInInput = ref(defaultExpiresIn)
const quotaValueInput = ref<number | undefined>(undefined)
const quotaUnitInput = ref<'MB' | 'GB'>('GB')
const quotaPeriodInput = ref('')

const editOwnerInput = ref('')
const editGroupIdsInput = ref<string[]>([])
const editInboundIdsInput = ref<string[]>([])
const editExpiresInInput = ref(defaultExpiresIn)
const editQuotaValueInput = ref<number | undefined>(undefined)
const editQuotaUnitInput = ref<'MB' | 'GB'>('GB')
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
const resetTrafficMutation = useResetTrafficToken({
  onSuccess: () => {
    invalidate()
  },
})
const { data: trafficData, isLoading: isTrafficLoading } = useTokenTraffic(
  trafficTokenId,
  'day',
  30
)

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
  quotaValueInput.value = undefined
  quotaUnitInput.value = 'GB'
  quotaPeriodInput.value = ''
  isIssueSubmitting.value = false
}

function toggleTokenSelection(tokenId: string) {
  const next = new Set(selectedTokenIDs.value)
  if (next.has(tokenId)) {
    next.delete(tokenId)
  } else {
    next.add(tokenId)
  }
  selectedTokenIDs.value = next
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
    quota_bytes:
      quotaValueInput.value && quotaUnitInput.value
        ? parseByteSize(quotaValueInput.value, quotaUnitInput.value)
        : undefined,
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

async function handleResetTraffic(token: Token) {
  const ok = await confirm({
    title: 'Reset traffic',
    message: `Reset traffic counter for ${token.owner}?`,
    variant: 'destructive',
  })
  if (!ok) return
  pendingResetTrafficId.value = token.id
  resetTrafficMutation.mutate(token.id, {
    onSettled: () => {
      pendingResetTrafficId.value = ''
    },
  })
}

async function handleBatchDeactivate() {
  if (selectedTokenIDs.value.size === 0) return
  const ok = await confirm({
    title: 'Deactivate selected tokens',
    message: `Deactivate ${selectedTokenIDs.value.size} selected tokens?`,
    variant: 'destructive',
  })
  if (!ok) return
  try {
    await batchDeactivateTokens(Array.from(selectedTokenIDs.value))
    toast.success('Tokens deactivated')
    selectedTokenIDs.value = new Set()
    invalidate()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to deactivate tokens', { description: msg })
  }
}

async function handleBatchRemove() {
  if (selectedTokenIDs.value.size === 0) return
  const ok = await confirm({
    title: 'Remove selected tokens',
    message: `Remove ${selectedTokenIDs.value.size} selected tokens permanently?`,
    variant: 'destructive',
  })
  if (!ok) return
  try {
    await batchRemoveTokens(Array.from(selectedTokenIDs.value))
    toast.success('Tokens removed')
    selectedTokenIDs.value = new Set()
    invalidate()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to remove tokens', { description: msg })
  }
}

function openTrafficDialog(token: Token) {
  trafficTokenId.value = token.id
  showTrafficDialog.value = true
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

async function copyText(value: string) {
  if (!value) return
  try {
    await navigator.clipboard?.writeText(value)
  } catch {
    // clipboard may be unavailable outside secure context
  }
}

function formatRelativeTime(date: Date): string {
  const now = Date.now()
  const diff = now - date.getTime()
  const abs = Math.abs(diff)
  const seconds = Math.floor(abs / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (diff < 0) {
    // future
    if (days > 30) return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
    if (days > 0) return `in ${days}d`
    if (hours > 0) return `in ${hours}h`
    if (minutes > 0) return `in ${minutes}m`
    return 'soon'
  }

  if (seconds < 60) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  if (hours < 24) return `${hours}h ago`
  if (days < 7) return `${days}d ago`
  if (days < 30) return `${Math.floor(days / 7)}w ago`
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
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
  if (token.quota_bytes) {
    const size = bytesToSize(token.quota_bytes)
    editQuotaValueInput.value = size.value
    editQuotaUnitInput.value = size.unit
  } else {
    editQuotaValueInput.value = undefined
    editQuotaUnitInput.value = 'GB'
  }
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
  editQuotaValueInput.value = undefined
  editQuotaUnitInput.value = 'GB'
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
    quota_bytes:
      editQuotaValueInput.value && editQuotaUnitInput.value
        ? parseByteSize(editQuotaValueInput.value, editQuotaUnitInput.value)
        : undefined,
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

    <div v-if="selectedTokenIDs.size > 0" class="flex items-center gap-2">
      <span class="text-sm font-medium">{{ selectedTokenIDs.size }} selected</span>
      <UiButton size="sm" variant="destructive" @click="handleBatchDeactivate">Deactivate</UiButton>
      <UiButton size="sm" variant="destructive" @click="handleBatchRemove">Remove</UiButton>
      <UiButton size="sm" variant="outline" @click="selectedTokenIDs = new Set()">Clear</UiButton>
    </div>

    <div v-if="isLoading" class="py-8 text-center text-muted-foreground">Loading tokens...</div>
    <div v-else-if="sortedTokens.length === 0" class="py-8 text-center text-muted-foreground">
      No tokens issued yet
    </div>

    <UiCard v-for="token in sortedTokens" :key="token.id" class="p-4">
      <CardContent class="p-0">
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-2 min-w-0">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-input shrink-0"
              :checked="selectedTokenIDs.has(token.id)"
              @change="toggleTokenSelection(token.id)"
            />
            <span
              class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
              :class="statusBadge(token).cls"
            >
              {{ statusBadge(token).label }}
            </span>
            <span class="truncate text-base font-semibold">{{ token.owner }}</span>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <UiButton variant="ghost" size="icon" class="shrink-0 -mr-2 -mt-1">
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
              <DropdownMenuItem
                @click="
                  ipRestrictionTokenId = token.id
                  showIPRestrictionsDialog = true
                "
              >
                <Shield class="h-4 w-4 mr-2" />
                IP Restrictions
              </DropdownMenuItem>
              <DropdownMenuItem @click="openTrafficDialog(token)">
                <BarChart3 class="h-4 w-4 mr-2" />
                View activity
              </DropdownMenuItem>
              <DropdownMenuItem
                :disabled="pendingResetTrafficId === token.id"
                @click="handleResetTraffic(token)"
              >
                <RotateCcw class="h-4 w-4 mr-2" />
                Reset traffic
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

        <div class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
          <div class="flex items-center gap-1.5 min-w-0">
            <Globe class="h-3.5 w-3.5 shrink-0 opacity-60" />
            <span class="text-[10px] text-muted-foreground/50 shrink-0">Groups</span>
            <span class="truncate text-muted-foreground">{{ tokenGroupLabels(token) }}</span>
          </div>
          <div class="flex items-center gap-1.5 min-w-0">
            <ArrowLeftRight class="h-3.5 w-3.5 shrink-0 opacity-60" />
            <span class="text-[10px] text-muted-foreground/50 shrink-0">Inbounds</span>
            <span class="truncate text-muted-foreground">{{ tokenInboundLabels(token) }}</span>
          </div>
          <div class="col-span-2 flex items-center gap-1.5 min-w-0">
            <Activity class="h-3.5 w-3.5 shrink-0 opacity-60" />
            <span class="text-[10px] text-muted-foreground/50 shrink-0">Traffic</span>
            <span class="shrink-0 font-medium text-foreground">{{
              formatBytes(token.used_bytes)
            }}</span>
            <template v-if="token.quota_bytes">
              <div class="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
                <div
                  class="h-full rounded-full transition-all"
                  :class="token.used_bytes / token.quota_bytes >= 0.9 ? 'bg-red-500' : 'bg-primary'"
                  :style="{
                    width: `${Math.min(100, Math.round((token.used_bytes / token.quota_bytes) * 100))}%`,
                  }"
                />
              </div>
              <span class="shrink-0 text-muted-foreground">{{
                formatBytes(token.quota_bytes)
              }}</span>
            </template>
            <span v-else-if="token.quota_period" class="shrink-0 text-muted-foreground">{{
              periodLabel(token.quota_period)
            }}</span>
          </div>
          <div class="flex items-center gap-1.5 min-w-0">
            <Wifi class="h-3.5 w-3.5 shrink-0 opacity-60" />
            <span class="text-[10px] text-muted-foreground/50 shrink-0">Last seen</span>
            <span class="text-muted-foreground">
              {{
                token.last_connected_at
                  ? formatRelativeTime(new Date(token.last_connected_at))
                  : 'Never'
              }}
            </span>
          </div>
          <div class="flex items-center gap-1.5 min-w-0">
            <Calendar class="h-3.5 w-3.5 shrink-0 opacity-60" />
            <span class="text-[10px] text-muted-foreground/50 shrink-0">Expires</span>
            <span class="text-muted-foreground">{{
              formatRelativeTime(new Date(token.expires_at))
            }}</span>
          </div>
        </div>

        <div class="mt-2 text-[10px] text-muted-foreground/60">
          Created {{ new Date(token.created_at).toLocaleDateString('en-US') }}
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
              <label class="text-sm font-medium">Quota</label>
              <UiInput v-model.number="quotaValueInput" type="number" placeholder="e.g. 10" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Unit</label>
              <select
                v-model="quotaUnitInput"
                class="w-full rounded-md border bg-background px-3 py-2 text-sm"
              >
                <option value="MB">MB</option>
                <option value="GB">GB</option>
              </select>
            </div>
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Quota Period</label>
            <div class="flex gap-2">
              <label
                class="flex-1 cursor-pointer rounded-md border px-3 py-2 text-center text-sm transition-colors"
                :class="
                  quotaPeriodInput === ''
                    ? 'border-primary bg-primary/10'
                    : 'bg-background hover:bg-accent'
                "
              >
                <input v-model="quotaPeriodInput" type="radio" value="" class="sr-only" />
                No limit
              </label>
              <label
                class="flex-1 cursor-pointer rounded-md border px-3 py-2 text-center text-sm transition-colors"
                :class="
                  quotaPeriodInput === 'day'
                    ? 'border-primary bg-primary/10'
                    : 'bg-background hover:bg-accent'
                "
              >
                <input v-model="quotaPeriodInput" type="radio" value="day" class="sr-only" />
                Daily
              </label>
              <label
                class="flex-1 cursor-pointer rounded-md border px-3 py-2 text-center text-sm transition-colors"
                :class="
                  quotaPeriodInput === 'month'
                    ? 'border-primary bg-primary/10'
                    : 'bg-background hover:bg-accent'
                "
              >
                <input v-model="quotaPeriodInput" type="radio" value="month" class="sr-only" />
                Monthly
              </label>
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
              <label class="text-sm font-medium">Quota</label>
              <UiInput v-model.number="editQuotaValueInput" type="number" placeholder="e.g. 10" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Unit</label>
              <select
                v-model="editQuotaUnitInput"
                class="w-full rounded-md border bg-background px-3 py-2 text-sm"
              >
                <option value="MB">MB</option>
                <option value="GB">GB</option>
              </select>
            </div>
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">Quota Period</label>
            <div class="flex gap-2">
              <label
                class="flex-1 cursor-pointer rounded-md border px-3 py-2 text-center text-sm transition-colors"
                :class="
                  editQuotaPeriodInput === ''
                    ? 'border-primary bg-primary/10'
                    : 'bg-background hover:bg-accent'
                "
              >
                <input v-model="editQuotaPeriodInput" type="radio" value="" class="sr-only" />
                No limit
              </label>
              <label
                class="flex-1 cursor-pointer rounded-md border px-3 py-2 text-center text-sm transition-colors"
                :class="
                  editQuotaPeriodInput === 'day'
                    ? 'border-primary bg-primary/10'
                    : 'bg-background hover:bg-accent'
                "
              >
                <input v-model="editQuotaPeriodInput" type="radio" value="day" class="sr-only" />
                Daily
              </label>
              <label
                class="flex-1 cursor-pointer rounded-md border px-3 py-2 text-center text-sm transition-colors"
                :class="
                  editQuotaPeriodInput === 'month'
                    ? 'border-primary bg-primary/10'
                    : 'bg-background hover:bg-accent'
                "
              >
                <input v-model="editQuotaPeriodInput" type="radio" value="month" class="sr-only" />
                Monthly
              </label>
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

    <Sheet v-model:open="showTrafficDialog">
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Activity</SheetTitle>
          <SheetDescription>Daily traffic for the selected token.</SheetDescription>
        </SheetHeader>
        <div class="py-4">
          <div v-if="isTrafficLoading" class="py-8 text-center text-muted-foreground">
            Loading chart...
          </div>
          <TokenTrafficChart v-else-if="trafficData?.length" :items="trafficData" />
          <div v-else class="py-8 text-center text-muted-foreground">No activity data yet</div>
        </div>
        <SheetFooter>
          <UiButton variant="outline" @click="showTrafficDialog = false">Close</UiButton>
        </SheetFooter>
      </SheetContent>
    </Sheet>

    <TokenIPRestrictions v-model:open="showIPRestrictionsDialog" :token-id="ipRestrictionTokenId" />
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
