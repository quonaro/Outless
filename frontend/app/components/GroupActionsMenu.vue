<script setup lang="ts">
import { Menu, Pencil, Play, Trash2 } from 'lucide-vue-next'
import type { Group } from '~/utils/schemas/group'
import UiButton from '~/components/ui/button/button.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'

const props = defineProps<{
  group: Group
  isDeleting?: boolean
  isRunning?: boolean
}>()

const emit = defineEmits<{
  (e: 'edit' | 'delete' | 'start-top-up', group: Group): void
}>()
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <UiButton variant="outline" size="icon" class="h-8 w-8 shrink-0">
        <Menu class="h-4 w-4" />
        <span class="sr-only">Group actions</span>
      </UiButton>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem @click="emit('edit', props.group)">
        <Pencil class="h-4 w-4 mr-2" />
        Edit
      </DropdownMenuItem>
      <DropdownMenuItem
        v-if="props.group.is_topup && props.group.top_up_id"
        :disabled="props.isRunning"
        @click="emit('start-top-up', props.group)"
      >
        <Play class="h-4 w-4 mr-2" />
        {{ props.isRunning ? 'Running...' : 'Start top-up' }}
      </DropdownMenuItem>
      <DropdownMenuItem
        class="text-red-600 focus:bg-red-50 focus:text-red-600"
        :disabled="props.isDeleting"
        @click="emit('delete', props.group)"
      >
        <Trash2 class="h-4 w-4 mr-2" />
        {{ props.isDeleting ? 'Deleting...' : 'Delete' }}
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
