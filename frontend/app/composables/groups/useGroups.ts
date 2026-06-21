import { useQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { fetchGroups } from '~/utils/services/group'
import type { Group } from '~/utils/schemas/group'

export function useGroups(options?: UseQueryOptions<Group[], Error>) {
  return useQuery({
    queryKey: ['groups'],
    queryFn: () => fetchGroups(),
    ...options,
  })
}
