import { useQuery, useMutation, useQueryClient, type UseQueryOptions } from '@tanstack/vue-query'
import { fetchInbounds, createInbound, updateInbound, deleteInbound } from '~/utils/services/inbound'
import type { Inbound, CreateInbound, UpdateInbound } from '~/utils/schemas/inbound'

export function useInbounds(options?: UseQueryOptions<Inbound[], Error>) {
  return useQuery({
    queryKey: ['inbounds'],
    queryFn: () => fetchInbounds(),
    ...options,
  })
}

export function useCreateInbound() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (inbound: CreateInbound) => createInbound(inbound),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inbounds'] })
    },
  })
}

export function useUpdateInbound() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateInbound }) => updateInbound(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inbounds'] })
    },
  })
}

export function useDeleteInbound() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => deleteInbound(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inbounds'] })
    },
  })
}
