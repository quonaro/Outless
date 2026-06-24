<script setup lang="ts">
import { computed } from 'vue'
import type { Node } from '~/utils/schemas/node'
import type { Inbound } from '~/utils/schemas/inbound'
import { parseVlessUrl } from '~/utils/vless'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import UiButton from '~/components/ui/button/button.vue'
import {
  MoreHorizontal,
  Tags,
  Trash2,
  type LucideIcon,
  Tag,
  Globe,
  KeyRound,
  Shield,
  Lock,
  Zap,
  Wifi,
  Server,
  Fingerprint,
  Key,
  Hash,
  Layers,
  Link,
  ArrowUpRight,
  Package,
  Sparkles,
  Monitor,
  Network,
} from 'lucide-vue-next'

interface Field {
  label: string
  value: string
}

interface FieldMeta {
  icon: LucideIcon
  colorClass: string
}

function getFieldMeta(label: string): FieldMeta {
  switch (label) {
    case 'Name':
      return { icon: Tag, colorClass: 'text-slate-500' }
    case 'Type':
      return { icon: Monitor, colorClass: 'text-slate-500' }
    case 'Host':
      return { icon: Globe, colorClass: 'text-blue-500' }
    case 'Port':
      return { icon: Network, colorClass: 'text-blue-500' }
    case 'UUID':
      return { icon: KeyRound, colorClass: 'text-violet-500' }
    case 'Security':
      return { icon: Shield, colorClass: 'text-amber-500' }
    case 'Encryption':
      return { icon: Lock, colorClass: 'text-yellow-500' }
    case 'Flow':
      return { icon: Zap, colorClass: 'text-emerald-500' }
    case 'Network':
      return { icon: Wifi, colorClass: 'text-cyan-500' }
    case 'SNI':
      return { icon: Server, colorClass: 'text-rose-500' }
    case 'Fingerprint':
      return { icon: Fingerprint, colorClass: 'text-pink-500' }
    case 'Public Key':
      return { icon: Key, colorClass: 'text-indigo-500' }
    case 'Short ID':
      return { icon: Hash, colorClass: 'text-zinc-500' }
    case 'ALPN':
      return { icon: Layers, colorClass: 'text-teal-500' }
    case 'Path':
      return { icon: Link, colorClass: 'text-green-500' }
    case 'Host Header':
      return { icon: ArrowUpRight, colorClass: 'text-sky-500' }
    case 'Service Name':
      return { icon: Package, colorClass: 'text-lime-500' }
    case 'SPX':
      return { icon: Sparkles, colorClass: 'text-fuchsia-500' }
    default:
      return { icon: Tag, colorClass: 'text-muted-foreground' }
  }
}

const props = defineProps<{
  node: Node
  inbounds?: Inbound[]
  groupLabel?: string
  deleting?: boolean
  showActions?: boolean
}>()

const emit = defineEmits<{
  editGroups: [node: Node]
  deleteNode: [node: Node]
}>()

const parsed = computed(() => {
  if (props.node.is_self || !props.node.url) {
    return null
  }
  return parseVlessUrl(props.node.url)
})

const selfInbound = computed(() => {
  if (!props.node.is_self || !props.inbounds || props.inbounds.length === 0) {
    return null
  }
  return props.inbounds[0]
})

function buildFields(): Field[] {
  const ib = selfInbound.value
  const p = parsed.value

  if (props.node.is_self) {
    return [
      { label: 'Host', value: ib?.address || '—' },
      { label: 'Port', value: ib ? String(ib.port || 443) : '—' },
      { label: 'UUID', value: '—' },
      { label: 'Security', value: 'reality' },
      { label: 'Encryption', value: 'none' },
      { label: 'Flow', value: 'xtls-rprx-vision' },
      { label: 'Network', value: 'tcp' },
      { label: 'SNI', value: ib?.sni || '—' },
      {
        label: 'Fingerprint',
        value: ib && ib.fingerprint !== 'random' ? ib.fingerprint : '—',
      },
      { label: 'Public Key', value: ib?.public_key || '—' },
      { label: 'Short ID', value: ib?.short_id || '—' },
    ]
  }

  if (!p) {
    return [{ label: 'URL', value: props.node.url || '—' }]
  }

  return [
    { label: 'Host', value: p.host || '—' },
    { label: 'Port', value: String(p.port) },
    { label: 'UUID', value: p.uuid },
    { label: 'Security', value: p.security || 'none' },
    { label: 'Encryption', value: p.encryption || 'none' },
    { label: 'Flow', value: p.flow || '—' },
    { label: 'Network', value: p.network || 'tcp' },
    { label: 'SNI', value: p.sni || '—' },
    { label: 'Fingerprint', value: p.fp || '—' },
    { label: 'Public Key', value: p.pbk || '—' },
    { label: 'Short ID', value: p.sid || '—' },
    ...(p.alpn.length > 0 ? [{ label: 'ALPN', value: p.alpn.join(', ') }] : []),
    ...(p.path ? [{ label: 'Path', value: p.path }] : []),
    ...(p.hostHeader ? [{ label: 'Host Header', value: p.hostHeader }] : []),
    ...(p.service ? [{ label: 'Service Name', value: p.service }] : []),
    ...(p.spx ? [{ label: 'SPX', value: p.spx }] : []),
  ]
}

const fields = computed<Field[]>(() => buildFields())

const title = computed(() => {
  if (props.node.is_self) return 'Current Machine'
  const p = parsed.value
  return p?.name || props.node.url
})
</script>

<template>
  <UiCard class="px-3 py-2">
    <CardContent class="p-0">
      <div class="flex items-start gap-2">
        <div class="min-w-0 flex-1 space-y-2">
          <div class="flex items-center gap-2">
            <p class="truncate text-sm font-medium">
              {{ title }}
            </p>
            <span
              v-if="node.is_self"
              class="inline-flex shrink-0 items-center rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
            >
              Self
            </span>
          </div>

          <div class="grid grid-cols-1 gap-x-4 gap-y-1.5 text-sm sm:grid-cols-2 lg:grid-cols-3">
            <div v-for="f in fields" :key="f.label" class="flex min-w-0 items-center gap-1.5">
              <component
                :is="getFieldMeta(f.label).icon"
                :class="['h-4 w-4 shrink-0', getFieldMeta(f.label).colorClass]"
              />
              <span class="shrink-0 text-muted-foreground">{{ f.label }}:</span>
              <span class="min-w-0 truncate font-medium" :title="f.value">{{ f.value }}</span>
            </div>
          </div>

          <p v-if="groupLabel" class="text-xs text-muted-foreground">
            {{ node.id }} · {{ groupLabel }}
          </p>
          <p v-else class="text-xs text-muted-foreground">
            {{ node.id }}
          </p>
        </div>

        <div
          v-if="showActions"
          class="flex shrink-0 flex-nowrap items-start justify-end gap-1 whitespace-nowrap"
        >
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <UiButton variant="ghost" size="icon" class="h-7 w-7" @click.prevent>
                <MoreHorizontal class="h-3.5 w-3.5" />
              </UiButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem @click.prevent="emit('editGroups', node)">
                <Tags class="mr-2 h-3.5 w-3.5" />
                Groups
              </DropdownMenuItem>
              <DropdownMenuItem
                class="text-destructive focus:text-destructive"
                :disabled="deleting"
                @click.prevent="emit('deleteNode', node)"
              >
                <Trash2 class="mr-2 h-3.5 w-3.5" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </CardContent>
  </UiCard>
</template>
