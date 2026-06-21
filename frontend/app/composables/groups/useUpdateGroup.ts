import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { updateGroup } from '~/utils/services/group'
import type { UpdateGroup } from '~/utils/schemas/group'

export function useUpdateGroup(
  options?: UseMutationOptions<void, Error, { id: string; group: UpdateGroup }>
) {
  return useMutation({
    mutationFn: ({ id, group }: { id: string; group: UpdateGroup }) =>
      updateGroup(id, group),
    onSuccess: () => {
      toast.success('Group updated successfully')
    },
    onError: (err) => {
      toast.error('Failed to update group', {
        description: err.message,
      })
    },
    ...options,
  })
}
