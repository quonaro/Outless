import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import {
  fetchTokenTrafficStats,
  fetchNodeTrafficStats,
  fetchInboundTrafficStats,
  fetchDomainTrafficStats,
  fetchDomainHistory,
  clearDomainHistory,
} from '~/utils/services/stats'
import type { DomainHierarchyOutput, EntityTrafficOutput } from '~/utils/schemas/stats'

type TrafficQueryOptions<T> = Omit<UseQueryOptions<T, Error>, 'queryKey' | 'queryFn'>

export function useTokenTrafficStats(options?: TrafficQueryOptions<EntityTrafficOutput>) {
  return useQuery({
    queryKey: ['token-traffic-stats'],
    queryFn: () => fetchTokenTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}

export function useNodeTrafficStats(options?: TrafficQueryOptions<EntityTrafficOutput>) {
  return useQuery({
    queryKey: ['node-traffic-stats'],
    queryFn: () => fetchNodeTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}

export function useInboundTrafficStats(options?: TrafficQueryOptions<EntityTrafficOutput>) {
  return useQuery({
    queryKey: ['inbound-traffic-stats'],
    queryFn: () => fetchInboundTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}

export function useDomainTrafficStats(options?: TrafficQueryOptions<EntityTrafficOutput>) {
  return useQuery({
    queryKey: ['domain-traffic-stats'],
    queryFn: () => fetchDomainTrafficStats(),
    refetchInterval: 30_000,
    ...options,
  })
}

export function useDomainHistory(days = 30, options?: TrafficQueryOptions<DomainHierarchyOutput>) {
  return useQuery({
    queryKey: ['domain-history', days],
    queryFn: () => fetchDomainHistory(days),
    refetchInterval: 30_000,
    ...options,
  })
}

export function useClearDomainHistory(options?: UseMutationOptions<void, Error, void>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => clearDomainHistory(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domain-history'] })
      toast.success('Domain history cleared')
    },
    onError: (err) => {
      toast.error('Failed to clear domain history', {
        description: err.message,
      })
    },
    ...options,
  })
}
