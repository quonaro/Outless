<script setup lang="ts">
import { computed, ref } from 'vue'
import { Upload, FileJson } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import UiButton from '~/components/ui/button/button.vue'
import ImportGroupDialog from '~/components/ImportGroupDialog.vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import { useGroups } from '~/composables/groups/useGroups'
import { useCreateGroup } from '~/composables/groups/useCreateGroup'
import type { Group } from '~/utils/schemas/group'

interface ParsedImportData {
  nodes: Record<string, unknown>[]
  groups: Record<string, unknown>[]
}

const emit = defineEmits<{
  (e: 'imported'): void
}>()

const { $api } = useNuxtApp()
const { data: groups, isLoading: isGroupsLoading } = useGroups()
const createGroupMutation = useCreateGroup()
const isCreatingGroup = computed(() => createGroupMutation.isPending.value)

const fileInput = ref<HTMLInputElement | null>(null)
const importMode = ref<'checker' | 'backup'>('checker')
const showGroupDialog = ref(false)
const pendingData = ref<ParsedImportData | null>(null)

function openFileInput(mode: 'checker' | 'backup') {
  importMode.value = mode
  fileInput.value?.click()
}

function normalizeNodes(nodes: Record<string, unknown>[], groupId: string) {
  return nodes.map((node) => ({
    ...node,
    group_ids: [groupId],
  }))
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  try {
    const text = await file.text()
    const data = JSON.parse(text)

    if (!Array.isArray(data.nodes) || !Array.isArray(data.groups)) {
      throw new Error('Invalid backup file: expected "nodes" and "groups" arrays')
    }

    if (importMode.value === 'checker') {
      pendingData.value = {
        nodes: data.nodes as Record<string, unknown>[],
        groups: data.groups as Record<string, unknown>[],
      }
      showGroupDialog.value = true
      return
    }

    await $api('/v1/import', {
      method: 'POST',
      body: data,
    })

    toast.success('Configuration imported')
    emit('imported')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Import failed', { description: msg })
  } finally {
    input.value = ''
  }
}

async function handleImportConfirm(
  payload: { mode: 'existing'; groupId: string } | { mode: 'new'; name: string }
) {
  if (!pendingData.value) return

  try {
    let targetGroup: Group | null = null
    let groupsToImport: Record<string, unknown>[] = []

    if (payload.mode === 'new') {
      targetGroup = await createGroupMutation.mutateAsync({
        name: payload.name,
        random_enabled: false,
        show_origins: false,
      })
      groupsToImport = [targetGroup]
    } else {
      const existing = groups.value?.find((g) => g.id === payload.groupId)
      if (!existing) throw new Error('Selected group not found')
      targetGroup = existing
    }

    const body = {
      groups: groupsToImport,
      nodes: normalizeNodes(pendingData.value.nodes, targetGroup.id),
    }

    await $api('/v1/import', {
      method: 'POST',
      body,
    })

    toast.success('Nodes imported from checker')
    emit('imported')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Import failed', { description: msg })
  } finally {
    showGroupDialog.value = false
    pendingData.value = null
  }
}
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <UiButton variant="outline" class="w-36 whitespace-nowrap">
        <Upload class="h-4 w-4 mr-2" />
        Import
      </UiButton>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem @click="openFileInput('checker')">
        <FileJson class="h-4 w-4 mr-2" />
        From checker
      </DropdownMenuItem>
      <DropdownMenuItem @click="openFileInput('backup')">
        <FileJson class="h-4 w-4 mr-2" />
        From backup
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
  <input ref="fileInput" type="file" accept=".json" class="hidden" @change="handleFileChange" />
  <ImportGroupDialog
    v-model="showGroupDialog"
    :groups="groups ?? []"
    :is-loading="isGroupsLoading"
    :is-creating="isCreatingGroup"
    @confirm="handleImportConfirm"
  />
</template>
