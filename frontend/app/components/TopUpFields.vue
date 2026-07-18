<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { TopUpFormValues } from '~/utils/services/group'
import { pingSourceURL } from '~/utils/services/group'
import UiLabel from '~/components/ui/label/label.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiButton from '~/components/ui/button/button.vue'
import UiSelect from '~/components/ui/select/select.vue'
import {
  Plus,
  Trash2,
  Check,
  Loader2,
  AlertTriangle,
  Link,
  Code,
  Clock,
  Hourglass,
  Calendar,
  Power,
  Activity,
  Users,
  Timer,
  Flag,
  Zap,
  Layers,
} from 'lucide-vue-next'

const topUpForm = defineModel<TopUpFormValues>({ required: true })

interface PingStatus {
  state: 'loading' | 'ok' | 'error'
  message?: string
}

interface SourceUrlItem {
  id: number
  value: string
  status?: PingStatus
}

const urlCounter = ref(0)
const sourceUrls = ref<SourceUrlItem[]>([])

onMounted(() => {
  const initial = topUpForm.value.urlsText
    .split('\n')
    .map((u) => u.trim())
    .filter((u) => u !== '')
  sourceUrls.value = initial.length
    ? initial.map((value) => ({ id: urlCounter.value++, value }))
    : [{ id: urlCounter.value++, value: '' }]
})

const parserOptions = [
  { label: 'VLESS lines', value: 'vless_lines' },
  { label: 'Base64', value: 'base64' },
  { label: 'Clash YAML', value: 'clash_yaml' },
]

const stageOptions = ['port', 'handshake', 'proxy']

const enabled = computed({
  get: () => topUpForm.value.enabled,
  set: (v) => (topUpForm.value = { ...topUpForm.value, enabled: v }),
})

const checkEnabled = computed({
  get: () => topUpForm.value.checkEnabled,
  set: (v) => (topUpForm.value = { ...topUpForm.value, checkEnabled: v }),
})

const scheduleExprDate = computed(() => {
  if (topUpForm.value.scheduleType !== 'fixed' || !topUpForm.value.scheduleExpr) {
    return ''
  }
  const d = new Date(topUpForm.value.scheduleExpr)
  if (Number.isNaN(d.getTime())) {
    return ''
  }
  const pad = (n: number) => n.toString().padStart(2, '0')
  const year = d.getFullYear()
  const month = pad(d.getMonth() + 1)
  const day = pad(d.getDate())
  const hours = pad(d.getHours())
  const minutes = pad(d.getMinutes())
  return `${year}-${month}-${day}T${hours}:${minutes}`
})

function updateScheduleExprFromDate(value: string | number) {
  const v = typeof value === 'number' ? String(value) : value
  if (!v) {
    updateField('scheduleExpr', '')
    return
  }
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) {
    return
  }
  updateField('scheduleExpr', d.toISOString())
}

function updateField<K extends keyof TopUpFormValues>(field: K, value: TopUpFormValues[K]) {
  topUpForm.value = { ...topUpForm.value, [field]: value }
}

function syncUrlsText() {
  updateField(
    'urlsText',
    sourceUrls.value
      .map((item) => item.value.trim())
      .filter((u) => u !== '')
      .join('\n')
  )
}

function addSourceUrl() {
  sourceUrls.value.push({ id: urlCounter.value++, value: '' })
}

function removeSourceUrl(id: number) {
  sourceUrls.value = sourceUrls.value.filter((item) => item.id !== id)
  syncUrlsText()
}

async function pingUrl(item: SourceUrlItem) {
  const url = item.value.trim()
  if (!url) {
    item.status = undefined
    return
  }
  item.status = { state: 'loading' }
  try {
    const result = await pingSourceURL(url)
    if (result.ok) {
      item.status = { state: 'ok', message: `OK ${result.latency_ms}ms` }
    } else {
      item.status = { state: 'error', message: result.error || 'Unavailable' }
    }
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    item.status = { state: 'error', message }
  }
}

function handleUrlBlur(item: SourceUrlItem) {
  syncUrlsText()
  pingUrl(item)
}

function toggleStage(stage: string) {
  const stages = new Set(topUpForm.value.stages)
  if (stages.has(stage)) {
    stages.delete(stage)
  } else {
    stages.add(stage)
  }
  updateField('stages', Array.from(stages))
}
</script>

<template>
  <div class="space-y-2">
    <UiLabel class="flex items-center gap-2">
      <Link class="h-4 w-4 text-muted-foreground" />
      Source URLs
    </UiLabel>
    <div class="space-y-2">
      <div v-for="item in sourceUrls" :key="item.id" class="flex items-start gap-2">
        <div class="flex-1 space-y-1">
          <UiInput
            v-model="item.value"
            placeholder="https://example.com/subscription"
            @blur="handleUrlBlur(item)"
            @keyup.enter="handleUrlBlur(item)"
          />
          <div v-if="item.status" class="flex items-center gap-1.5 text-xs">
            <Loader2
              v-if="item.status.state === 'loading'"
              class="h-3.5 w-3.5 animate-spin text-muted-foreground"
            />
            <Check v-else-if="item.status.state === 'ok'" class="h-3.5 w-3.5 text-emerald-500" />
            <AlertTriangle
              v-else-if="item.status.state === 'error'"
              class="h-3.5 w-3.5 text-red-500"
            />
            <span
              :class="{
                'text-emerald-600 dark:text-emerald-400': item.status.state === 'ok',
                'text-red-600 dark:text-red-400': item.status.state === 'error',
                'text-muted-foreground': item.status.state === 'loading',
              }"
            >
              {{ item.status.message }}
            </span>
          </div>
        </div>
        <UiButton
          v-if="sourceUrls.length > 1"
          variant="outline"
          size="icon"
          type="button"
          @click="removeSourceUrl(item.id)"
        >
          <Trash2 class="h-4 w-4" />
        </UiButton>
      </div>
    </div>
    <UiButton variant="outline" size="sm" type="button" class="mt-2" @click="addSourceUrl">
      <Plus class="h-4 w-4 mr-2" />
      Add URL
    </UiButton>
  </div>

  <div class="space-y-2">
    <UiLabel class="flex items-center gap-2">
      <Code class="h-4 w-4 text-muted-foreground" />
      Parser
    </UiLabel>
    <UiSelect
      :model-value="topUpForm.parserType"
      :options="parserOptions"
      @update:model-value="updateField('parserType', $event as string)"
    />
  </div>

  <div class="space-y-2">
    <UiLabel class="flex items-center gap-2">
      <Clock class="h-4 w-4 text-muted-foreground" />
      Schedule Type
    </UiLabel>
    <UiSelect
      :model-value="topUpForm.scheduleType"
      :options="[
        { label: 'Interval', value: 'interval' },
        { label: 'Fixed time', value: 'fixed' },
      ]"
      @update:model-value="updateField('scheduleType', $event as string)"
    />
  </div>

  <div v-if="topUpForm.scheduleType === 'interval'" class="space-y-2">
    <UiLabel class="flex items-center gap-2">
      <Hourglass class="h-4 w-4 text-muted-foreground" />
      Schedule Expression
    </UiLabel>
    <UiInput
      :model-value="topUpForm.scheduleExpr"
      placeholder="1h"
      @update:model-value="updateField('scheduleExpr', $event as string)"
    />
  </div>

  <div v-else class="space-y-2">
    <UiLabel class="flex items-center gap-2">
      <Calendar class="h-4 w-4 text-muted-foreground" />
      Schedule Time
    </UiLabel>
    <UiInput
      type="datetime-local"
      :model-value="scheduleExprDate"
      placeholder="2026-01-01T00:00"
      @update:model-value="updateScheduleExprFromDate"
    />
  </div>

  <div class="flex items-center gap-2">
    <input id="top-up-enabled" v-model="enabled" type="checkbox" class="h-4 w-4" />
    <UiLabel for="top-up-enabled" class="flex items-center gap-2">
      <Power class="h-4 w-4 text-muted-foreground" />
      Enabled
    </UiLabel>
  </div>

  <div class="flex items-center gap-2">
    <input id="top-up-check" v-model="checkEnabled" type="checkbox" class="h-4 w-4" />
    <UiLabel for="top-up-check" class="flex items-center gap-2">
      <Activity class="h-4 w-4 text-muted-foreground" />
      Run availability checks
    </UiLabel>
  </div>

  <template v-if="topUpForm.checkEnabled">
    <div class="space-y-2">
      <UiLabel class="flex items-center gap-2">
        <Users class="h-4 w-4 text-muted-foreground" />
        Workers
      </UiLabel>
      <UiInput
        :model-value="topUpForm.workers"
        type="number"
        @update:model-value="updateField('workers', Number($event))"
      />
    </div>
    <div class="space-y-2">
      <UiLabel class="flex items-center gap-2">
        <Timer class="h-4 w-4 text-muted-foreground" />
        Timeout
      </UiLabel>
      <UiInput
        :model-value="topUpForm.timeout"
        placeholder="5s"
        @update:model-value="updateField('timeout', $event as string)"
      />
    </div>
    <div class="space-y-2">
      <UiLabel class="flex items-center gap-2">
        <Flag class="h-4 w-4 text-muted-foreground" />
        Exclude Countries (comma separated)
      </UiLabel>
      <UiInput
        :model-value="topUpForm.excludeCountries"
        placeholder="RU, CN"
        @update:model-value="updateField('excludeCountries', $event as string)"
      />
    </div>
    <div class="space-y-2">
      <UiLabel class="flex items-center gap-2">
        <Zap class="h-4 w-4 text-muted-foreground" />
        Max Latency
      </UiLabel>
      <UiInput
        :model-value="topUpForm.maxLatency"
        placeholder="2s"
        @update:model-value="updateField('maxLatency', $event as string)"
      />
    </div>
    <div class="space-y-2">
      <UiLabel class="flex items-center gap-2">
        <Layers class="h-4 w-4 text-muted-foreground" />
        Check Stages
      </UiLabel>
      <div class="flex gap-4">
        <label v-for="stage in stageOptions" :key="stage" class="flex items-center gap-1 text-sm">
          <input
            type="checkbox"
            :checked="topUpForm.stages.includes(stage)"
            @change="toggleStage(stage)"
          />
          {{ stage }}
        </label>
      </div>
    </div>
  </template>
</template>
