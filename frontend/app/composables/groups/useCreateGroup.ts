import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { createGroup } from '~/utils/services/group'
import type { Group, CreateGroup } from '~/utils/schemas/group'

export function useCreateGroup(
  options?: UseMutationOptions<Group, Error, CreateGroup>
) {
  return useMutation({
    mutationFn: (group: CreateGroup) => createGroup(group),
    onSuccess: () => {
      toast.success('Group created successfully')
    },
    onError: (err) => {
      toast.error('Failed to create group', {
        description: err.message,
      })
    },
    ...options,
  })
}
