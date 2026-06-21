<script setup lang="ts">
import type { DialogContentEmits, DialogContentProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { X } from 'lucide-vue-next'
import {
  DialogClose,
  DialogContent,
  DialogOverlay,
  DialogPortal,
  injectDialogRootContext,
  useForwardPropsEmits,
} from 'reka-ui'
import { cn } from '~/utils'

const props = defineProps<DialogContentProps & { class?: HTMLAttributes['class'] }>()
const emits = defineEmits<DialogContentEmits>()

const delegatedProps = reactiveOmit(props, 'class')

const forwarded = useForwardPropsEmits(delegatedProps, emits)

const rootContext = injectDialogRootContext()

const SWIPE_THRESHOLD = 80
let touchStartY = 0
let touchStartX = 0

function onTouchStart(event: TouchEvent) {
  const touch = event.touches[0]
  if (!touch) return
  touchStartY = touch.clientY
  touchStartX = touch.clientX
}

function onTouchEnd(event: TouchEvent) {
  const touch = event.changedTouches[0]
  if (!touch) return
  const deltaY = touch.clientY - touchStartY
  const deltaX = Math.abs(touch.clientX - touchStartX)
  if (deltaY > SWIPE_THRESHOLD && deltaX < deltaY * 0.8) {
    rootContext?.onOpenChange(false)
  }
}
</script>

<template>
  <DialogPortal>
    <DialogOverlay
      class="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
    />
    <DialogContent
      v-bind="forwarded"
      :class="
        cn(
          'fixed z-50 flex flex-col gap-4 border bg-background p-6 shadow-lg',
          // Mobile bottom sheet
          'inset-x-0 bottom-0 h-auto w-full max-w-full rounded-t-xl border-t data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=open]:slide-in-from-bottom-full data-[state=closed]:slide-out-to-bottom-full data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 duration-300',
          'max-h-[85dvh] overflow-y-auto sm:max-h-none sm:overflow-visible',
          // Desktop centered dialog overrides
          'sm:inset-auto sm:left-1/2 sm:top-1/2 sm:w-full sm:max-w-lg sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-lg sm:border-border sm:border sm:duration-200 sm:data-[state=open]:slide-in-from-left-1/2 sm:data-[state=open]:slide-in-from-top-[48%] sm:data-[state=closed]:slide-out-to-left-1/2 sm:data-[state=closed]:slide-out-to-top-[48%] sm:data-[state=open]:zoom-in-95 sm:data-[state=closed]:zoom-out-95',
          props.class
        )
      "
    >
      <div
        class="mx-auto mb-2 h-1.5 w-12 shrink-0 rounded-full bg-muted sm:hidden touch-pan-y"
        @touchstart="onTouchStart"
        @touchend="onTouchEnd"
      />
      <slot />
      <DialogClose
        class="absolute right-4 top-4 rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none data-[state=open]:bg-accent data-[state=open]:text-muted-foreground"
      >
        <X class="h-4 w-4" />
        <span class="sr-only">Close</span>
      </DialogClose>
    </DialogContent>
  </DialogPortal>
</template>
