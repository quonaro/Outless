import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { createNode } from '~/utils/services/node'
import type { CreateNode } from '~/utils/schemas/node'

export function useCreateNode(
  options?: UseMutationOptions<void, Error, CreateNode>,
) {
  return useMutation({
    mutationFn: (payload: CreateNode) => createNode(payload),
    onSuccess: () => {
      toast.success('Node created successfully')
    },
    onError: (err) => {
      toast.error('Failed to create node', {
        description: err.message,
      })
    },
    ...options,
  })
}
