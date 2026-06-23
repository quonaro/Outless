import { useQuery } from '@tanstack/vue-query'
import { toValue, type Ref, computed } from 'vue'
import { fetchTokenTraffic } from '~/utils/services/token'

export function useTokenTraffic(
  id: string | Ref<string>,
  period: 'day' | 'month' = 'day',
  limit = 30
) {
  return useQuery({
    queryKey: ['tokenTraffic', id, period, limit],
    queryFn: () => fetchTokenTraffic(toValue(id), period, limit),
    enabled: computed(() => !!toValue(id)),
  })
}
