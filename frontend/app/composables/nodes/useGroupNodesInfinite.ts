import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useInfiniteQuery } from '@tanstack/vue-query'
import { fetchNodesPage, type NodesPage } from '~/utils/services/node'

/** Matches global list API default page size for grouped accordion chunks. */
export const GROUP_NODES_PAGE_SIZE = 30

export function useGroupNodesInfinite(groupId: MaybeRefOrGetter<string>) {
  const gid = computed(() => toValue(groupId))

  return useInfiniteQuery<NodesPage, Error>({
    queryKey: computed(() => ['nodes', 'group', gid.value, 'infinite'] as const),
    initialPageParam: 0,
    enabled: computed(() => Boolean(gid.value)),
    queryFn: ({ pageParam }) =>
      fetchNodesPage(GROUP_NODES_PAGE_SIZE, Number(pageParam), gid.value),
    getNextPageParam: (lastPage) =>
      lastPage.hasMore && lastPage.nextOffset != null ? lastPage.nextOffset : undefined,
  })
}
