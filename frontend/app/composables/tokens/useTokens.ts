import { useQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { fetchTokens } from '~/utils/services/token'
import type { Token } from '~/utils/schemas/token'

export function useTokens(options?: UseQueryOptions<Token[], Error>) {
  return useQuery({
    queryKey: ['tokens'],
    queryFn: () => fetchTokens(),
    ...options,
  })
}
