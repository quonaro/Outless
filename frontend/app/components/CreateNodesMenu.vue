<script setup lang="ts">
import { Plus, Folder, Server } from 'lucide-vue-next'
import { computed } from 'vue'
import UiButton from '~/components/ui/button/button.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'

const props = defineProps<{
  groups?: { length: number } | null
}>()

const emit = defineEmits<{
  (e: 'create-group' | 'create-node'): void
}>()

const canCreateNode = computed(() => (props.groups?.length ?? 0) > 0)
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <UiButton class="w-36 whitespace-nowrap">
        <Plus class="h-4 w-4 mr-2" />
        Create
      </UiButton>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem @click="emit('create-group')">
        <Folder class="h-4 w-4 mr-2" />
        Group
      </DropdownMenuItem>
      <DropdownMenuItem
        :disabled="!canCreateNode"
        :title="canCreateNode ? '' : 'Create a group first'"
        @click="emit('create-node')"
      >
        <Server class="h-4 w-4 mr-2" />
        Node
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
