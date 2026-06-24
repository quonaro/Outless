<script setup lang="ts">
import { computed } from 'vue'
import { parseVlessUrl } from '~/utils/vless'
import {
  type LucideIcon,
  Globe,
  Network,
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
  Tag,
} from 'lucide-vue-next'

const props = defineProps<{
  url: string
}>()

interface FieldMeta {
  icon: LucideIcon
  colorClass: string
}

function getFieldMeta(label: string): FieldMeta {
  switch (label) {
    case 'Name':
      return { icon: Tag, colorClass: 'text-slate-500' }
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

const parsed = computed(() => {
  const trimmed = props.url.trim()
  if (!trimmed || !trimmed.startsWith('vless://')) return null
  return parseVlessUrl(trimmed)
})

const fields = computed(() => {
  const p = parsed.value
  if (!p) return []

  const list = [
    { label: 'Name', value: p.name || '—' },
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
  ]
  if (p.alpn.length > 0) list.push({ label: 'ALPN', value: p.alpn.join(', ') })
  if (p.path) list.push({ label: 'Path', value: p.path })
  if (p.hostHeader) list.push({ label: 'Host Header', value: p.hostHeader })
  if (p.service) list.push({ label: 'Service Name', value: p.service })
  if (p.spx) list.push({ label: 'SPX', value: p.spx })
  return list
})
</script>

<template>
  <div v-if="parsed" class="rounded-md border bg-muted/30 px-3 py-2 space-y-1.5">
    <p class="text-xs font-medium text-muted-foreground">Parsed VLESS URL</p>
    <div class="grid grid-cols-1 gap-x-4 gap-y-1 text-xs sm:grid-cols-2">
      <div v-for="f in fields" :key="f.label" class="flex min-w-0 items-center gap-1.5">
        <component
          :is="getFieldMeta(f.label).icon"
          :class="['h-3.5 w-3.5 shrink-0', getFieldMeta(f.label).colorClass]"
        />
        <span class="shrink-0 text-muted-foreground">{{ f.label }}:</span>
        <span class="min-w-0 truncate font-medium" :title="f.value">{{ f.value }}</span>
      </div>
    </div>
  </div>
</template>
