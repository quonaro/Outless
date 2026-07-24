<script setup lang="ts">
import { computed } from 'vue'
import { PinInputRoot, PinInputInput } from 'radix-vue'
import { cn } from '~/utils'

interface Props {
  id?: string
  name?: string
  modelValue?: string
  length?: number
  placeholder?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  id: undefined,
  name: undefined,
  length: 6,
  modelValue: '',
  placeholder: '',
  disabled: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const digits = computed({
  get: () =>
    Array.from({ length: props.length }, (_, i) =>
      props.modelValue && props.modelValue[i] ? props.modelValue[i] : ''
    ),
  set: (value) => emit('update:modelValue', value.slice(0, props.length).join('')),
})
</script>

<template>
  <PinInputRoot
    :id="id"
    v-model="digits"
    :name="name"
    :placeholder="placeholder"
    :disabled="disabled"
    otp
    type="text"
    class="flex items-center justify-center gap-2"
  >
    <template v-for="i in length" :key="i">
      <PinInputInput
        :index="i - 1"
        inputmode="numeric"
        pattern="[0-9]*"
        :class="
          cn(
            'h-12 w-12 rounded-md border border-input bg-input text-center text-2xl font-semibold text-foreground shadow-sm',
            'transition-colors placeholder:text-muted-foreground',
            'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring focus-visible:ring-inset',
            'disabled:cursor-not-allowed disabled:opacity-50'
          )
        "
      />
      <div
        v-if="i === 3"
        class="text-2xl font-semibold text-muted-foreground select-none"
        aria-hidden="true"
      >
        -
      </div>
    </template>
  </PinInputRoot>
</template>
