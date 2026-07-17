import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useInfiniteQuery } from '@tanstack/vue-query'
import { fetchNodesPage, type NodesPage } from '~/utils/services/node'

const PAGE_LIMIT = 50

export function useInfiniteNodes(
  enabled: MaybeRefOrGetter<boolean> = true,
  groupId: MaybeRefOrGetter<string> = ''
) {
  const enabledRef = computed(() => toValue(enabled) !== false)
  const groupIdRef = computed(() => toValue(groupId))

  return useInfiniteQuery<NodesPage, Error>({
    queryKey: computed(() => ['nodes', 'infinite', groupIdRef.value]),
    initialPageParam: 0,
    enabled: enabledRef,
    queryFn: ({ pageParam }) => fetchNodesPage(PAGE_LIMIT, Number(pageParam), groupIdRef.value),
    getNextPageParam: (lastPage) =>
      lastPage.hasMore && lastPage.nextOffset != null ? lastPage.nextOffset : undefined,
  })
}
