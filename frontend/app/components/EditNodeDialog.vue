<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Node } from '~/utils/schemas/node'
import type { Group } from '~/utils/schemas/group'
import type { ParsedVless } from '~/utils/vless'
import { parseVlessUrl, buildVlessUrl, updateVlessName } from '~/utils/vless'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import NodeLifetimeInput from '~/components/NodeLifetimeInput.vue'
import { Unlock } from 'lucide-vue-next'

const props = defineProps<{
  modelValue: boolean
  node?: Node | null
  groups: Group[]
  saving?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [payload: { url?: string; groupIds: string[]; expiresAt?: string }]
}>()

const localParsed = ref<ParsedVless | null>(null)
const localGroupIds = ref<string[]>([])
const localExpiresAt = ref<string | undefined>(undefined)
const originalUrl = ref('')
const originalExpiresAt = ref<string | undefined>(undefined)
const rawUrl = ref('')
const isRaw = ref(false)
const unlocked = ref(false)
const confirmingUnlock = ref(false)

const portInput = computed({
  get: () => String(localParsed.value?.port ?? 443),
  set: (value) => {
    if (localParsed.value) {
      const n = parseInt(value, 10)
      localParsed.value.port = Number.isNaN(n) ? 443 : n
    }
  },
})

watch(
  () => props.modelValue,
  (open) => {
    if (open && props.node) {
      const parsed = parseVlessUrl(props.node.url)
      localParsed.value = parsed
      isRaw.value = parsed === null
      rawUrl.value = props.node.url
      originalUrl.value = props.node.url
      localGroupIds.value = [...props.node.group_ids]
      localExpiresAt.value = props.node.expires_at
      originalExpiresAt.value = props.node.expires_at
      unlocked.value = false
      confirmingUnlock.value = false
    }
    if (!open) {
      localParsed.value = null
      localGroupIds.value = []
      localExpiresAt.value = undefined
      originalUrl.value = ''
      originalExpiresAt.value = undefined
      rawUrl.value = ''
      isRaw.value = false
      unlocked.value = false
      confirmingUnlock.value = false
    }
  },
  { immediate: true }
)

function startUnlock() {
  confirmingUnlock.value = true
}

function confirmUnlock() {
  confirmingUnlock.value = false
  unlocked.value = true
}

function cancelUnlock() {
  confirmingUnlock.value = false
}

function submit() {
  const payload: { url?: string; groupIds: string[]; expiresAt?: string } = {
    groupIds: localGroupIds.value,
  }

  if (isRaw.value) {
    const newUrl = updateVlessName(rawUrl.value, localParsed.value?.name ?? '')
    if (!newUrl) {
      alert('Invalid VLESS URL')
      return
    }
    if (newUrl !== originalUrl.value) payload.url = newUrl
  } else if (localParsed.value) {
    const built = buildVlessUrl(localParsed.value)
    if (built !== originalUrl.value) payload.url = built
  }

  if (localExpiresAt.value !== originalExpiresAt.value) {
    payload.expiresAt = localExpiresAt.value
  }

  emit('save', payload)
}
</script>

<template>
  <Sheet :open="modelValue" @update:open="emit('update:modelValue', $event)">
    <SheetContent>
      <SheetHeader>
        <SheetTitle>Edit Node</SheetTitle>
        <SheetDescription>Name, groups and expiration are always editable.</SheetDescription>
      </SheetHeader>
      <div v-if="node" class="space-y-4 py-4">
        <div class="space-y-2">
          <label class="text-sm font-medium" for="edit-node-name">Name</label>
          <UiInput
            v-if="localParsed"
            id="edit-node-name"
            v-model="localParsed.name"
            placeholder="Node name"
            @keyup.enter="submit"
          />
          <UiInput v-else id="edit-node-name" :model-value="''" disabled placeholder="Node name" />
        </div>
        <template v-if="localParsed && !isRaw">
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-medium">Host</label>
              <UiInput v-model="localParsed.host" :disabled="!unlocked" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Port</label>
              <UiInput v-model="portInput" type="number" :disabled="!unlocked" />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-medium">Security</label>
              <UiInput v-model="localParsed.security" :disabled="!unlocked" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Network</label>
              <UiInput v-model="localParsed.network" :disabled="!unlocked" />
            </div>
          </div>
          <div class="space-y-2">
            <label class="text-sm font-medium">UUID</label>
            <UiInput v-model="localParsed.uuid" :disabled="!unlocked" />
          </div>
        </template>
        <div v-else-if="isRaw" class="space-y-2">
          <label class="text-sm font-medium">URL</label>
          <UiInput v-model="rawUrl" :disabled="!unlocked" />
        </div>
        <div class="space-y-2">
          <NodeLifetimeInput v-model="localExpiresAt" />
        </div>
        <div class="space-y-2">
          <label class="text-sm font-medium">Groups</label>
          <div
            class="max-h-32 overflow-y-auto rounded-md border bg-background px-3 py-2 text-sm space-y-1"
          >
            <label v-for="g in groups" :key="g.id" class="flex items-center gap-2 cursor-pointer">
              <input
                v-model="localGroupIds"
                type="checkbox"
                :value="g.id"
                class="h-4 w-4 rounded border-input"
              />
              <span>{{ g.name }}</span>
            </label>
          </div>
        </div>
        <div v-if="!unlocked && (localParsed || isRaw)" class="rounded-md border p-3 text-sm">
          <p v-if="!confirmingUnlock" class="text-muted-foreground">
            Connection fields are locked. Unlock to edit host, port, security, network and UUID.
          </p>
          <div v-else class="space-y-2">
            <p class="text-destructive">
              Editing connection fields can break the node configuration. Are you sure?
            </p>
            <div class="flex items-center gap-2">
              <UiButton size="sm" variant="destructive" @click="confirmUnlock"
                >Yes, unlock</UiButton
              >
              <UiButton size="sm" variant="outline" @click="cancelUnlock">Cancel</UiButton>
            </div>
          </div>
          <UiButton
            v-if="!confirmingUnlock"
            variant="outline"
            size="sm"
            class="mt-2"
            @click="startUnlock"
          >
            <Unlock class="mr-1.5 h-3.5 w-3.5" />
            Unlock all fields
          </UiButton>
        </div>
        <div v-else-if="unlocked" class="text-xs text-destructive">
          Connection fields are unlocked.
        </div>
      </div>
      <SheetFooter>
        <UiButton variant="outline" @click="emit('update:modelValue', false)">Cancel</UiButton>
        <UiButton :disabled="saving" @click="submit">
          {{ saving ? 'Saving...' : 'Save' }}
        </UiButton>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>
