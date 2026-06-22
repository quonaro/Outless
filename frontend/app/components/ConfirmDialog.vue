<script setup lang="ts">
import { useConfirm } from '~/composables/useConfirm'
import UiButton from '~/components/ui/button/button.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'

const { state, onConfirm, onCancel } = useConfirm()
</script>

<template>
  <Sheet
    :open="state.isOpen"
    @update:open="
      (v: boolean) => {
        if (!v) onCancel()
      }
    "
  >
    <SheetContent>
      <SheetHeader>
        <SheetTitle>{{ state.title }}</SheetTitle>
        <SheetDescription>{{ state.message }}</SheetDescription>
      </SheetHeader>
      <SheetFooter>
        <UiButton variant="outline" @click="onCancel">
          {{ state.cancelLabel }}
        </UiButton>
        <UiButton
          :variant="state.variant === 'destructive' ? 'destructive' : 'default'"
          @click="onConfirm"
        >
          {{ state.confirmLabel }}
        </UiButton>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>
