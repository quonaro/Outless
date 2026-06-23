import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { resetTokenTraffic } from '~/utils/services/token'

export function useResetTrafficToken(options?: UseMutationOptions<void, Error, string>) {
  return useMutation({
    mutationFn: (id: string) => resetTokenTraffic(id),
    onSuccess: () => {
      toast.success('Traffic counter reset successfully')
    },
    onError: (err) => {
      toast.error('Failed to reset traffic counter', {
        description: err.message,
      })
    },
    ...options,
  })
}
