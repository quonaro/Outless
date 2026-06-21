import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { createPublicSource } from '~/utils/services/public-source'
import type { CreatePublicSource, PublicSource } from '~/utils/schemas/public-source'

export function useCreatePublicSource(
  options?: UseMutationOptions<PublicSource, Error, CreatePublicSource>,
) {
  return useMutation({
    mutationFn: (payload: CreatePublicSource) =>
      createPublicSource(payload),
    onSuccess: () => {
      toast.success('Public source created successfully')
    },
    onError: (err) => {
      toast.error('Failed to create public source', {
        description: err.message,
      })
    },
    ...options,
  })
}
