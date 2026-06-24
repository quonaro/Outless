<script setup lang="ts">
import { ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { Shield, Plus, Trash2 } from 'lucide-vue-next'
import {
  fetchTokenIPRestrictions,
  addTokenIPRestriction,
  removeTokenIPRestriction,
  type IPRestriction,
} from '~/utils/services/token'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'

const props = defineProps<{
  tokenId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const restrictions = ref<IPRestriction[]>([])
const isLoading = ref(false)
const newIP = ref('')
const newMode = ref<'allow' | 'block'>('allow')
const isAdding = ref(false)

async function load() {
  if (!props.tokenId) return
  isLoading.value = true
  try {
    restrictions.value = await fetchTokenIPRestrictions(props.tokenId)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to load IP restrictions', { description: msg })
  } finally {
    isLoading.value = false
  }
}

async function handleAdd() {
  if (!newIP.value.trim() || isAdding.value) return
  isAdding.value = true
  try {
    await addTokenIPRestriction(props.tokenId, newIP.value.trim(), newMode.value)
    newIP.value = ''
    await load()
    toast.success('IP restriction added')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to add IP restriction', { description: msg })
  } finally {
    isAdding.value = false
  }
}

async function handleRemove(ip: string) {
  try {
    await removeTokenIPRestriction(props.tokenId, ip)
    await load()
    toast.success('IP restriction removed')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to remove IP restriction', { description: msg })
  }
}

watch(open, (v) => {
  if (v) load()
})
</script>

<template>
  <Sheet v-model:open="open">
    <SheetContent>
      <SheetHeader>
        <SheetTitle class="flex items-center gap-2">
          <Shield class="h-5 w-5" />
          IP Restrictions
        </SheetTitle>
        <SheetDescription> Manage IP allow/block rules for this token. </SheetDescription>
      </SheetHeader>

      <div class="space-y-4 py-4">
        <div class="flex gap-2">
          <UiInput v-model="newIP" placeholder="192.168.1.1" class="flex-1" />
          <select v-model="newMode" class="rounded-md border bg-background px-2 py-2 text-sm">
            <option value="allow">Allow</option>
            <option value="block">Block</option>
          </select>
          <UiButton :disabled="!newIP.trim() || isAdding" @click="handleAdd">
            <Plus class="h-4 w-4" />
          </UiButton>
        </div>

        <div v-if="isLoading" class="py-4 text-center text-muted-foreground">Loading...</div>
        <div v-else-if="restrictions.length === 0" class="py-4 text-center text-muted-foreground">
          No restrictions configured
        </div>
        <div v-else class="space-y-2">
          <UiCard v-for="r in restrictions" :key="r.ip" class="p-2">
            <CardContent class="p-0 flex items-center justify-between gap-2">
              <div class="min-w-0">
                <p class="text-sm font-medium truncate">{{ r.ip }}</p>
                <p class="text-xs text-muted-foreground capitalize">{{ r.mode }}</p>
              </div>
              <UiButton
                variant="ghost"
                size="icon"
                class="h-7 w-7 shrink-0"
                @click="handleRemove(r.ip)"
              >
                <Trash2 class="h-3.5 w-3.5 text-destructive" />
              </UiButton>
            </CardContent>
          </UiCard>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
