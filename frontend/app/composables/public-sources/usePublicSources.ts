import { useQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { fetchPublicSources } from '~/utils/services/public-source'
import type { PublicSource } from '~/utils/schemas/public-source'

export function usePublicSources(options?: UseQueryOptions<PublicSource[], Error>) {
  return useQuery({
    queryKey: ['public-sources'],
    queryFn: () => fetchPublicSources(),
    ...options,
  })
}
