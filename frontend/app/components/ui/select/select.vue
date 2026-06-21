<script setup lang="ts">
import type { HTMLAttributes } from "vue";
import { useVModel } from "@vueuse/core";
import { cn } from "~/utils";

interface SelectOption {
  label: string;
  value: string;
}

const props = defineProps<{
  modelValue?: string;
  options: SelectOption[];
  placeholder?: string;
  class?: HTMLAttributes["class"];
}>();

const emits = defineEmits<{
  (e: "update:modelValue", payload: string): void;
}>();

const modelValue = useVModel(props, "modelValue", emits, {
  passive: true,
});
</script>

<template>
  <select
    v-model="modelValue"
    :class="
      cn(
        'flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50',
        props.class,
      )
    "
  >
    <option v-if="placeholder" disabled value="">{{ placeholder }}</option>
    <option v-for="opt in options" :key="opt.value" :value="opt.value">
      {{ opt.label }}
    </option>
  </select>
</template>
