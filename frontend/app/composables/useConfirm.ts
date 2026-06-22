import { reactive } from 'vue'

export interface ConfirmOptions {
  title?: string
  message: string
  variant?: 'default' | 'destructive'
  confirmLabel?: string
  cancelLabel?: string
}

const state = reactive({
  isOpen: false,
  title: 'Confirm',
  message: '',
  variant: 'default' as 'default' | 'destructive',
  confirmLabel: 'OK',
  cancelLabel: 'Cancel',
})

let resolveFn: ((value: boolean) => void) | null = null

export function useConfirm() {
  function confirm(options: ConfirmOptions): Promise<boolean> {
    state.title = options.title ?? 'Confirm'
    state.message = options.message
    state.variant = options.variant ?? 'default'
    state.confirmLabel = options.confirmLabel ?? 'OK'
    state.cancelLabel = options.cancelLabel ?? 'Cancel'
    state.isOpen = true
    return new Promise((resolve) => {
      resolveFn = resolve
    })
  }

  function onConfirm() {
    state.isOpen = false
    resolveFn?.(true)
    resolveFn = null
  }

  function onCancel() {
    state.isOpen = false
    resolveFn?.(false)
    resolveFn = null
  }

  return { confirm, state, onConfirm, onCancel }
}
