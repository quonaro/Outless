import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { deleteGroup } from '~/utils/services/group'

export function useDeleteGroup(
  options?: UseMutationOptions<void, Error, string>
) {
  return useMutation({
    mutationFn: (id: string) => deleteGroup(id),
    onSuccess: () => {
      toast.success('Group deleted successfully')
    },
    onError: (err) => {
      toast.error('Failed to delete group', {
        description: err.message,
      })
    },
    ...options,
  })
}
