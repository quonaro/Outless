<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Group } from '~/utils/schemas/group'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiSelect from '~/components/ui/select/select.vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'

const props = defineProps<{
  modelValue: boolean
  groups: Group[]
  isLoading: boolean
  isCreating: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: [payload: { mode: 'existing'; groupId: string } | { mode: 'new'; name: string }]
}>()

const newGroupValue = '__new__'
const selectedValue = ref('')
const newGroupName = ref('')

const selectOptions = computed(() => [
  { label: 'Create new group', value: newGroupValue },
  ...props.groups.map((g) => ({ label: g.name, value: g.id })),
])

const isNewGroup = computed(() => selectedValue.value === newGroupValue)
const canConfirm = computed(() => {
  if (isNewGroup.value) return newGroupName.value.trim().length > 0
  return selectedValue.value !== ''
})

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      selectedValue.value = props.groups.length > 0 ? '' : newGroupValue
      newGroupName.value = ''
    }
  },
  { immediate: true }
)

function handleConfirm() {
  if (!canConfirm.value) return

  if (isNewGroup.value) {
    emit('confirm', { mode: 'new', name: newGroupName.value.trim() })
    return
  }

  emit('confirm', { mode: 'existing', groupId: selectedValue.value })
}

function handleClose() {
  emit('update:modelValue', false)
}
</script>

<template>
  <Dialog :open="modelValue" @update:open="emit('update:modelValue', $event)">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Import nodes</DialogTitle>
        <DialogDescription>
          Choose an existing group or create a new one for the imported nodes.
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-4">
        <div class="space-y-2">
          <label class="text-sm font-medium">Target group</label>
          <UiSelect
            v-model="selectedValue"
            :options="selectOptions"
            placeholder="Select a group"
            :disabled="isLoading"
          />
          <div v-if="isLoading" class="text-sm text-muted-foreground">Loading groups...</div>
        </div>

        <div v-if="isNewGroup" class="space-y-2">
          <label class="text-sm font-medium">New group name</label>
          <UiInput
            v-model="newGroupName"
            placeholder="Enter group name"
            :disabled="isCreating"
            @keyup.enter="handleConfirm"
          />
        </div>
      </div>

      <DialogFooter>
        <UiButton variant="outline" :disabled="isCreating" @click="handleClose"> Cancel </UiButton>
        <UiButton :disabled="!canConfirm || isCreating || isLoading" @click="handleConfirm">
          {{ isCreating ? 'Creating...' : 'Import' }}
        </UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
