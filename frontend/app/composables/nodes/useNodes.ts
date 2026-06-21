import { useQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { fetchNodes } from '~/utils/services/node'
import type { Node } from '~/utils/schemas/node'

export function useNodes(options?: UseQueryOptions<Node[], Error>) {
  return useQuery({
    queryKey: ['nodes'],
    queryFn: () => fetchNodes(),
    ...options,
  })
}
