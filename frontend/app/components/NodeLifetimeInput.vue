<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Clock } from 'lucide-vue-next'
import UiInput from '~/components/ui/input/input.vue'

interface Props {
  modelValue?: string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:modelValue', value?: string): void
}>()

type Mode = 'none' | 'days' | 'date'

const mode = ref<Mode>('none')
const days = ref<string>('7')
const hours = ref<string>('0')
const minutes = ref<string>('0')
const date = ref<string>('')
const now = ref(Date.now())
let intervalId: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  intervalId = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (intervalId) clearInterval(intervalId)
})

let isInternalUpdate = false

function formatCountdown(ms: number) {
  if (ms <= 0) return 'now'
  const totalSeconds = Math.floor(ms / 1000)
  const d = Math.floor(totalSeconds / 86400)
  const h = Math.floor((totalSeconds % 86400) / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  const s = totalSeconds % 60
  if (d > 0) return `in ${d}d ${h}h ${m}m`
  if (h > 0) return `in ${h}h ${m}m ${s}s`
  if (m > 0) return `in ${m}m ${s}s`
  return `in ${s}s`
}

const expirationInfo = computed(() => {
  if (mode.value === 'none') return null

  let target: Date
  if (mode.value === 'days') {
    const ds = parseInt(days.value, 10) || 0
    const hs = parseInt(hours.value, 10) || 0
    const ms = parseInt(minutes.value, 10) || 0
    if (ds <= 0 && hs <= 0 && ms <= 0) return null
    target = new Date(now.value)
    target.setDate(target.getDate() + ds)
    target.setHours(target.getHours() + hs)
    target.setMinutes(target.getMinutes() + ms)
  } else {
    if (!date.value) return null
    target = new Date(date.value)
    if (Number.isNaN(target.getTime())) return null
  }

  const diff = target.getTime() - now.value
  const dateStr = target.toLocaleDateString(undefined, { dateStyle: 'medium' })
  const timeStr = target.toLocaleTimeString(undefined, { timeStyle: 'short' })
  return `Will be deleted on ${dateStr} ${timeStr} (${formatCountdown(diff)})`
})

const options: { value: Mode; label: string }[] = [
  { value: 'none', label: 'Never' },
  { value: 'days', label: 'Days' },
  { value: 'date', label: 'Date' },
]

function parseValue(value?: string) {
  if (!value) {
    mode.value = 'none'
    days.value = '7'
    hours.value = '0'
    minutes.value = '0'
    date.value = ''
    return
  }
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    mode.value = 'none'
    days.value = '7'
    hours.value = '0'
    minutes.value = '0'
    date.value = ''
    return
  }
  mode.value = 'date'
  date.value = d.toISOString().slice(0, 10)
}

function emitValue() {
  isInternalUpdate = true
  if (mode.value === 'none') {
    emit('update:modelValue', undefined)
    return
  }
  if (mode.value === 'days') {
    const ds = parseInt(days.value, 10) || 0
    const hs = parseInt(hours.value, 10) || 0
    const ms = parseInt(minutes.value, 10) || 0
    if (ds <= 0 && hs <= 0 && ms <= 0) {
      emit('update:modelValue', undefined)
      return
    }
    const d = new Date()
    d.setDate(d.getDate() + ds)
    d.setHours(d.getHours() + hs)
    d.setMinutes(d.getMinutes() + ms)
    emit('update:modelValue', d.toISOString())
    return
  }
  if (!date.value) {
    emit('update:modelValue', undefined)
    return
  }
  const d = new Date(date.value)
  if (Number.isNaN(d.getTime())) {
    emit('update:modelValue', undefined)
    return
  }
  emit('update:modelValue', d.toISOString())
}

parseValue(props.modelValue)
watch(
  () => props.modelValue,
  (value) => {
    if (isInternalUpdate) {
      isInternalUpdate = false
      return
    }
    parseValue(value)
  }
)
watch([mode, days, hours, minutes, date], emitValue)
</script>

<template>
  <div class="space-y-2">
    <label class="inline-flex items-center gap-2 text-sm font-medium">
      <Clock class="h-4 w-4" />
      Lifetime
    </label>
    <p class="text-xs text-muted-foreground">
      The node will be automatically deleted when the lifetime expires.
    </p>
    <div class="grid grid-cols-3 gap-2 rounded-lg border bg-muted p-1">
      <button
        v-for="opt in options"
        :key="opt.value"
        type="button"
        class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
        :class="
          mode === opt.value
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:bg-background/50 hover:text-foreground'
        "
        @click="mode = opt.value"
      >
        {{ opt.label }}
      </button>
    </div>
    <div v-if="mode === 'days'" class="flex flex-wrap items-center gap-2">
      <UiInput
        v-model="days"
        type="number"
        min="0"
        placeholder="7"
        class="w-20 [-moz-appearance:_textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
      />
      <span class="text-sm text-muted-foreground">days</span>
      <UiInput
        v-model="hours"
        type="number"
        min="0"
        max="23"
        placeholder="0"
        class="w-20 [-moz-appearance:_textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
      />
      <span class="text-sm text-muted-foreground">hours</span>
      <UiInput
        v-model="minutes"
        type="number"
        min="0"
        max="59"
        placeholder="0"
        class="w-20 [-moz-appearance:_textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
      />
      <span class="text-sm text-muted-foreground">minutes</span>
    </div>
    <div v-else-if="mode === 'date'">
      <UiInput v-model="date" type="date" class="w-full" />
    </div>
    <p v-if="expirationInfo" class="text-sm text-destructive">{{ expirationInfo }}</p>
  </div>
</template>
