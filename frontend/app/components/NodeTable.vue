<script setup lang="ts">
import { computed } from 'vue'
import type { Node } from '~/utils/schemas/node'
import type { Inbound } from '~/utils/schemas/inbound'
import { parseVlessUrl } from '~/utils/vless'
import UiButton from '~/components/ui/button/button.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import { toast } from 'vue-sonner'
import { MoreHorizontal, Trash2, Clock, Monitor, Pencil, Copy } from 'lucide-vue-next'

const props = defineProps<{
  nodes: Node[]
  inbounds?: Inbound[]
  groupNameByID: Record<string, string>
  selectedNodeIDs?: Set<string>
  deletingNodeIDs?: Set<string>
}>()

const emit = defineEmits<{
  toggleSelection: [nodeId: string]
  deleteNode: [node: Node]
  editNode: [node: Node]
}>()

interface Row {
  node: Node
  title: string
  host: string
  port: string
  security: string
  network: string
  uuid: string
  groups: string
  expiresLabel: string
  expired: boolean
  expiringSoon: boolean
}

const selfInbound = computed<Inbound | null>(() => {
  if (!props.inbounds || props.inbounds.length === 0) return null
  return props.inbounds[0] ?? null
})

function formatExpiresAt(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const diff = d.getTime() - Date.now()
  if (diff <= 0) return 'Expired'
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days === 0) return 'Expires today'
  if (days === 1) return 'Expires in 1 day'
  return `Expires in ${days} days`
}

function buildRow(node: Node): Row {
  const expiresAt = node.expires_at ? new Date(node.expires_at) : null
  const expired = !!expiresAt && expiresAt.getTime() < Date.now()
  const daysLeft = expiresAt
    ? Math.max(0, Math.ceil((expiresAt.getTime() - Date.now()) / (1000 * 60 * 60 * 24)))
    : null
  const expiringSoon = !!daysLeft && daysLeft > 0 && daysLeft <= 3

  const groups = node.group_ids.map((id) => props.groupNameByID[id] ?? id).join(', ')

  if (node.is_self) {
    const ib = selfInbound.value
    return {
      node,
      title: 'Current Machine',
      host: ib?.address || '—',
      port: ib ? String(ib.port || 443) : '—',
      security: 'reality',
      network: 'tcp',
      uuid: '—',
      groups,
      expiresLabel: node.expires_at ? formatExpiresAt(node.expires_at) : '',
      expired: !!node.expires_at && expired,
      expiringSoon: !!node.expires_at && !expired && expiringSoon,
    }
  }

  const parsed = parseVlessUrl(node.url)
  if (!parsed) {
    return {
      node,
      title: node.url,
      host: '—',
      port: '—',
      security: '—',
      network: '—',
      uuid: '—',
      groups,
      expiresLabel: node.expires_at ? formatExpiresAt(node.expires_at) : '',
      expired: !!node.expires_at && expired,
      expiringSoon: !!node.expires_at && !expired && expiringSoon,
    }
  }

  return {
    node,
    title: parsed.name || parsed.host,
    host: parsed.host || '—',
    port: String(parsed.port),
    security: parsed.security || 'none',
    network: parsed.network || 'tcp',
    uuid: parsed.uuid,
    groups,
    expiresLabel: node.expires_at ? formatExpiresAt(node.expires_at) : '',
    expired: !!node.expires_at && expired,
    expiringSoon: !!node.expires_at && !expired && expiringSoon,
  }
}

const rows = computed<Row[]>(() => props.nodes.map(buildRow))

function isSelected(nodeId: string): boolean {
  return props.selectedNodeIDs?.has(nodeId) ?? false
}

function isDeleting(nodeId: string): boolean {
  return props.deletingNodeIDs?.has(nodeId) ?? false
}

function onToggle(nodeId: string) {
  emit('toggleSelection', nodeId)
}

async function copyLink(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    toast.success('Link copied to clipboard')
  } catch {
    toast.error('Failed to copy link')
  }
}

function onToggleAll() {
  const allSelected = rows.value.every((row) => isSelected(row.node.id))
  for (const row of rows.value) {
    if (allSelected) {
      if (isSelected(row.node.id)) emit('toggleSelection', row.node.id)
    } else {
      if (!isSelected(row.node.id)) emit('toggleSelection', row.node.id)
    }
  }
}

const allVisibleSelected = computed(() => {
  if (rows.value.length === 0) return false
  return rows.value.every((row) => isSelected(row.node.id))
})
</script>

<template>
  <div class="overflow-auto rounded-md border">
    <table class="w-full text-sm">
      <thead class="bg-muted sticky top-0 z-10">
        <tr>
          <th class="w-10 px-2 py-2 text-center">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-input"
              :checked="allVisibleSelected"
              @change="onToggleAll"
            />
          </th>
          <th class="px-3 py-2 text-left font-medium">Host</th>
          <th class="px-3 py-2 text-left font-medium">Port</th>
          <th class="px-3 py-2 text-left font-medium">Security</th>
          <th class="px-3 py-2 text-left font-medium">Network</th>
          <th class="px-3 py-2 text-left font-medium">UUID</th>
          <th class="px-3 py-2 text-left font-medium">Groups</th>
          <th class="px-3 py-2 text-left font-medium">Expires</th>
          <th class="w-10 px-2 py-2"></th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.node.id"
          class="border-t transition-colors hover:bg-muted/40"
        >
          <td class="px-2 py-2 text-center">
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-input"
              :checked="isSelected(row.node.id)"
              @change="onToggle(row.node.id)"
            />
          </td>
          <td class="px-3 py-2">
            <div class="flex min-w-0 items-center gap-1.5">
              <Monitor v-if="row.node.is_self" class="h-3.5 w-3.5 shrink-0 text-emerald-500" />
              <div class="min-w-0">
                <div class="truncate font-medium" :title="row.title">{{ row.title }}</div>
                <div class="truncate text-xs text-muted-foreground" :title="row.host">
                  {{ row.host }}
                </div>
              </div>
            </div>
          </td>
          <td class="px-3 py-2 whitespace-nowrap font-medium">{{ row.port }}</td>
          <td class="px-3 py-2 whitespace-nowrap">{{ row.security }}</td>
          <td class="px-3 py-2 whitespace-nowrap">{{ row.network }}</td>
          <td class="px-3 py-2">
            <span class="block max-w-[12rem] truncate font-mono text-xs" :title="row.uuid">
              {{ row.uuid }}
            </span>
          </td>
          <td class="px-3 py-2">
            <span class="block max-w-[10rem] truncate text-xs" :title="row.groups">
              {{ row.groups || '—' }}
            </span>
          </td>
          <td class="px-3 py-2 whitespace-nowrap">
            <span
              v-if="row.expiresLabel"
              class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
              :class="{
                'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400': row.expired,
                'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400':
                  !row.expired && row.expiringSoon,
                'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400':
                  !row.expired && !row.expiringSoon,
              }"
            >
              <Clock class="h-3 w-3" />
              {{ row.expiresLabel }}
            </span>
            <span v-else class="text-muted-foreground">—</span>
          </td>
          <td class="px-2 py-2 text-right">
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <UiButton variant="ghost" size="icon" class="h-7 w-7" @click.prevent>
                  <MoreHorizontal class="h-3.5 w-3.5" />
                </UiButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  :disabled="row.node.is_self"
                  @click.prevent="emit('editNode', row.node)"
                >
                  <Pencil class="mr-2 h-3.5 w-3.5" />
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem :disabled="!row.node.url" @click.prevent="copyLink(row.node.url)">
                  <Copy class="mr-2 h-3.5 w-3.5" />
                  Copy link
                </DropdownMenuItem>
                <DropdownMenuItem
                  class="text-destructive focus:text-destructive"
                  :disabled="isDeleting(row.node.id)"
                  @click.prevent="emit('deleteNode', row.node)"
                >
                  <Trash2 class="mr-2 h-3.5 w-3.5" />
                  {{ isDeleting(row.node.id) ? 'Deleting...' : 'Delete' }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
