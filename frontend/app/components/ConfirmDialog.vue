<script setup lang="ts">
import { useConfirm } from '~/composables/useConfirm'
import UiButton from '~/components/ui/button/button.vue'
import Dialog from '~/components/ui/dialog/Dialog.vue'
import DialogContent from '~/components/ui/dialog/DialogContent.vue'
import DialogHeader from '~/components/ui/dialog/DialogHeader.vue'
import DialogTitle from '~/components/ui/dialog/DialogTitle.vue'
import DialogDescription from '~/components/ui/dialog/DialogDescription.vue'
import DialogFooter from '~/components/ui/dialog/DialogFooter.vue'

const { state, onConfirm, onCancel } = useConfirm()
</script>

<template>
  <Dialog :open="state.isOpen" @update:open="(v: boolean) => { if (!v) onCancel() }">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ state.title }}</DialogTitle>
        <DialogDescription>{{ state.message }}</DialogDescription>
      </DialogHeader>
      <DialogFooter>
        <UiButton variant="outline" @click="onCancel">
          {{ state.cancelLabel }}
        </UiButton>
        <UiButton
          :variant="state.variant === 'destructive' ? 'destructive' : 'default'"
          @click="onConfirm"
        >
          {{ state.confirmLabel }}
        </UiButton>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
