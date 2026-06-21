import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { updateNode } from '~/utils/services/node'
import type { UpdateNode } from '~/utils/schemas/node'

export interface UpdateNodePayload extends UpdateNode {
  id: string
}

export function useUpdateNode(
  options?: UseMutationOptions<void, Error, UpdateNodePayload>,
) {
  return useMutation({
    mutationFn: ({ id, ...rest }: UpdateNodePayload) =>
      updateNode(id, rest),
    onSuccess: () => {
      toast.success('Node updated successfully')
    },
    onError: (err) => {
      toast.error('Failed to update node', {
        description: err.message,
      })
    },
    ...options,
  })
}
