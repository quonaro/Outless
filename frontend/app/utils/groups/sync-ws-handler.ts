import type { QueryClient } from '@tanstack/vue-query'
import type { Ref, ShallowRef } from 'vue'
import {
  type GroupSyncDoneEvent,
  type GroupSyncNodeEvent,
  type GroupSyncStateEvent,
} from '~/utils/services/group'
import type { SyncNodeStatus } from '~/composables/groups/useGroupSync'

export interface SyncStateRefs {
  isSyncing: Ref<boolean>
  error: Ref<string>
  syncTotal: Ref<number>
  syncProcessed: Ref<number>
  syncAddedCount: Ref<number>
  syncedAt: Ref<string>
  isCancelled: Ref<boolean>
  syncingNodes: ShallowRef<Map<string, SyncNodeStatus>>
}

export function createWsHandler(
  groupId: string,
  refs: SyncStateRefs,
  queryClient: QueryClient,
  maybeUnsubscribe: () => void,
  refreshAfterSyncJob: () => void,
) {
  return function handleWsMessage(msg: Record<string, unknown>) {
    const t = msg.type
    if (t === 'sync_started') {
      if (msg.group_id && msg.group_id !== groupId) return
      refs.isSyncing.value = true
      if (typeof msg.total === 'number') refs.syncTotal.value = msg.total
      if (typeof msg.processed === 'number') refs.syncProcessed.value = msg.processed
      if (typeof msg.added_count === 'number') refs.syncAddedCount.value = msg.added_count
      return
    }
    if (t === 'sync_node_status') {
      const ev = msg as unknown as GroupSyncNodeEvent & { group_id?: string }
      if (ev.group_id && ev.group_id !== groupId) return
      const next = new Map(refs.syncingNodes.value)
      next.set(ev.node_id, {
        node_id: ev.node_id,
        url: ev.url,
        status: ev.status,
        error: ev.error,
      })
      refs.syncingNodes.value = next
      if (typeof ev.total === 'number') refs.syncTotal.value = ev.total
      if (typeof ev.processed === 'number') refs.syncProcessed.value = ev.processed
      if (typeof ev.added_total === 'number') refs.syncAddedCount.value = ev.added_total
      return
    }
    if (t === 'sync_group_state') {
      const ev = msg as unknown as GroupSyncStateEvent & { group_id?: string }
      if (ev.group_id && ev.group_id !== groupId) return
      refs.isSyncing.value = ev.running
      refs.syncProcessed.value = ev.processed
      refs.syncTotal.value = ev.total
      if (typeof ev.added_count === 'number') refs.syncAddedCount.value = ev.added_count
      if (ev.error) refs.error.value = ev.error
      if (ev.synced_at) refs.syncedAt.value = ev.synced_at
      const next = new Map<string, SyncNodeStatus>()
      for (const node of ev.nodes ?? []) {
        next.set(node.node_id, {
          node_id: node.node_id,
          url: node.url,
          status: node.status,
          error: node.error,
        })
      }
      refs.syncingNodes.value = next
      if (!ev.running) {
        maybeUnsubscribe()
      }
      return
    }
    if (t === 'sync_done') {
      const ev = msg as unknown as GroupSyncDoneEvent & { group_id?: string }
      if (ev.group_id && ev.group_id !== groupId) return
      refs.syncedAt.value = ev.synced_at
      if (typeof ev.total === 'number') refs.syncTotal.value = ev.total
      if (typeof ev.processed === 'number') refs.syncProcessed.value = ev.processed
      if (typeof ev.added_count === 'number') refs.syncAddedCount.value = ev.added_count
      refs.isSyncing.value = false
      refreshAfterSyncJob()
      maybeUnsubscribe()
      return
    }
    if (t === 'sync_error') {
      const errMsg = typeof msg.error === 'string' ? msg.error : 'sync failed'
      if (msg.group_id && msg.group_id !== groupId) return
      refs.error.value = errMsg
      refs.syncTotal.value = typeof msg.total === 'number' ? msg.total : refs.syncTotal.value
      refs.syncProcessed.value = typeof msg.processed === 'number' ? msg.processed : refs.syncProcessed.value
      refs.syncAddedCount.value = typeof msg.added_count === 'number' ? msg.added_count : refs.syncAddedCount.value
      refs.isSyncing.value = false
      refreshAfterSyncJob()
      maybeUnsubscribe()
      return
    }
    if (t === 'sync_cancelled') {
      if (msg.group_id && msg.group_id !== groupId) return
      refs.syncTotal.value = typeof msg.total === 'number' ? msg.total : refs.syncTotal.value
      refs.syncProcessed.value = typeof msg.processed === 'number' ? msg.processed : refs.syncProcessed.value
      refs.syncAddedCount.value = typeof msg.added_count === 'number' ? msg.added_count : refs.syncAddedCount.value
      refs.isCancelled.value = true
      refs.isSyncing.value = false
      refreshAfterSyncJob()
      maybeUnsubscribe()
    }
  }
}

