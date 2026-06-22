import { useQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { fetchTrafficStats } from '~/utils/services/stats'
import type { TrafficStats } from '~/utils/schemas/stats'

export function useTrafficStats(options?: UseQueryOptions<TrafficStats, Error>) {
  return useQuery({
    queryKey: ['traffic-stats'],
    queryFn: () => fetchTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}
