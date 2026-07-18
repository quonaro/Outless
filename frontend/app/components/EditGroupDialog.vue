<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import type { Group, UpdateGroup } from '~/utils/schemas/group'
import {
  updateGroup,
  fetchTopUp,
  buildTopUpInput,
  defaultTopUpForm,
  type TopUpFormValues,
} from '~/utils/services/group'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiLabel from '~/components/ui/label/label.vue'
import TopUpFields from '~/components/TopUpFields.vue'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetDescription,
} from '~/components/ui/sheet'

const props = defineProps<{
  group: Group | null
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const queryClient = useQueryClient()

const name = ref('')
const randomEnabled = ref(false)
const randomLimit = ref<number | undefined>(undefined)
const showTopUp = ref(false)
const topUpForm = ref<TopUpFormValues>(defaultTopUpForm())
const isTopUpLoading = ref(false)

const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: string; data: UpdateGroup }) => updateGroup(id, data),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    close()
  },
})

const isSubmitting = computed(() => updateMutation.isPending.value)

watch(
  () => props.open,
  (open) => {
    if (!open || !props.group) return
    name.value = props.group.name
    randomEnabled.value = props.group.random_enabled ?? false
    randomLimit.value = props.group.random_limit ?? undefined
    showTopUp.value = props.group.is_topup ?? false
    topUpForm.value = defaultTopUpForm()
    if (props.group.is_topup && props.group.top_up_id) {
      loadTopUp(props.group.top_up_id)
    }
  }
)

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

function buildPayload(): UpdateGroup {
  const payload: UpdateGroup = {
    name: name.value,
    random_enabled: randomEnabled.value,
    random_limit: randomLimit.value ?? null,
  }
  if (showTopUp.value) {
    payload.top_up = buildTopUpInput(topUpForm.value)
  }
  return payload
}

function save() {
  if (!props.group || !name.value.trim()) return
  updateMutation.mutate({ id: props.group.id, data: buildPayload() })
}

function close() {
  emit('update:open', false)
}
</script>

<template>
  <Sheet :open="props.open" @update:open="emit('update:open', $event)">
    <SheetContent>
      <SheetHeader>
        <SheetTitle>Edit Group</SheetTitle>
        <SheetDescription>Update group settings.</SheetDescription>
      </SheetHeader>
      <div v-if="isTopUpLoading" class="py-8 text-center text-muted-foreground">Loading...</div>
      <div v-else class="grow min-h-0 space-y-4 overflow-y-auto py-4">
        <div class="space-y-2">
          <UiLabel>Group Name</UiLabel>
          <UiInput v-model="name" placeholder="Enter group name" />
        </div>

        <div class="flex items-center gap-2">
          <input id="edit-random-enabled" v-model="randomEnabled" type="checkbox" class="h-4 w-4" />
          <UiLabel for="edit-random-enabled">Random selection for subscriptions</UiLabel>
        </div>

        <div class="space-y-2">
          <UiLabel>Limit (optional)</UiLabel>
          <UiInput v-model="randomLimit" type="number" min="1" placeholder="Max nodes to return" />
          <p class="text-xs text-muted-foreground">
            Maximum number of nodes to return in subscriptions
          </p>
        </div>

        <div v-if="!props.group?.is_topup" class="flex items-center gap-2">
          <input id="edit-top-up" v-model="showTopUp" type="checkbox" class="h-4 w-4" />
          <UiLabel for="edit-top-up">Self-refilling (top-up) group</UiLabel>
        </div>

        <TopUpFields v-if="showTopUp" v-model="topUpForm" />
      </div>
      <SheetFooter>
        <UiButton variant="outline" @click="close">Cancel</UiButton>
        <UiButton :disabled="!name.trim() || isSubmitting" @click="save">
          {{ isSubmitting ? 'Updating...' : 'Update' }}
        </UiButton>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>
