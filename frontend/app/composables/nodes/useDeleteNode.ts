import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { deleteNode } from '~/utils/services/node'

export function useDeleteNode(
  options?: UseMutationOptions<void, Error, string>
) {
  return useMutation({
    mutationFn: (id: string) => deleteNode(id),
    onSuccess: () => {
      toast.success('Node deleted successfully')
    },
    onError: (err) => {
      toast.error('Failed to delete node', {
        description: err.message,
      })
    },
    ...options,
  })
}
