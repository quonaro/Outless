import { useMutation, type UseMutationOptions } from '@tanstack/vue-query'
import { reissueToken } from '~/utils/services/token'
import type { ReissueResult } from '~/utils/services/token'

export function useReissueToken(options?: UseMutationOptions<ReissueResult, Error, string>) {
  return useMutation({
    mutationFn: (id: string) => reissueToken(id),
    ...options,
  })
}
