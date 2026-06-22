import { useQuery, type UseQueryOptions } from '@tanstack/vue-query'
import {
  fetchTokenTrafficStats,
  fetchNodeTrafficStats,
  fetchInboundTrafficStats,
} from '~/utils/services/stats'
import type { EntityTrafficOutput } from '~/utils/schemas/stats'

export function useTokenTrafficStats(options?: UseQueryOptions<EntityTrafficOutput, Error>) {
  return useQuery({
    queryKey: ['token-traffic-stats'],
    queryFn: () => fetchTokenTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}

export function useNodeTrafficStats(options?: UseQueryOptions<EntityTrafficOutput, Error>) {
  return useQuery({
    queryKey: ['node-traffic-stats'],
    queryFn: () => fetchNodeTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}

export function useInboundTrafficStats(options?: UseQueryOptions<EntityTrafficOutput, Error>) {
  return useQuery({
    queryKey: ['inbound-traffic-stats'],
    queryFn: () => fetchInboundTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}
