import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { fetchSettings, updateSettings } from '~/utils/services/settings'
import type { UpdateSettings } from '~/utils/schemas/settings'

export function useSettings() {
  return useQuery({
    queryKey: ['settings'],
    queryFn: fetchSettings,
  })
}

export function useUpdateSettings() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (settings: UpdateSettings) => updateSettings(settings),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
    },
  })
}
