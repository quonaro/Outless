import { useQueryClient } from '@tanstack/vue-query'
import { ref, shallowRef } from 'vue'
import { toast } from 'vue-sonner'
import {
  type GroupSyncNodeEvent,
  type GroupSyncStateEvent,
  type GroupSyncDoneEvent,
} from '~/utils/services/group'
import {
  ensureSSEConnected,
  sendSSECommand,
  subscribeGroupSyncChannel,
} from '~/composables/useSSE'

export interface SyncNodeStatus {
  node_id: string
  url: string
  status: 'importing' | 'done' | 'error'
  error?: string
}

export function useGroupSync(groupId: string) {
  const queryClient = useQueryClient()

  /** Refresh nodes/groups after source sync jobs (import may add/remove nodes). */
  function refreshAfterSyncJob() {
    void queryClient.invalidateQueries({ queryKey: ['nodes'] })
    void queryClient.invalidateQueries({ queryKey: ['nodes', 'infinite'] })
    void queryClient.invalidateQueries({ queryKey: ['groups'] })
  }

  const isSyncing = ref(false)
  const syncError = ref('')
  const syncTotal = ref(0)
  const syncProcessed = ref(0)
  const syncAddedCount = ref(0)
  const syncedAt = ref('')
  const isCancelled = ref(false)
  const syncingNodes = shallowRef<Map<string, SyncNodeStatus>>(new Map())

  let unsubscribe: (() => void) | null = null

  function ensureSubscribed() {
    ensureSSEConnected()
    if (!unsubscribe) {
      unsubscribe = subscribeGroupSyncChannel(groupId, handleWsMessage)
    }
  }

  function maybeUnsubscribe() {
    if (isSyncing.value) return
    if (unsubscribe) {
      unsubscribe()
      unsubscribe = null
    }
  }

  function handleWsMessage(msg: Record<string, unknown>) {
    const t = msg.type
    if (t === 'sync_started') {
      if (msg.group_id && msg.group_id !== groupId) return
      isSyncing.value = true
      if (typeof msg.total === 'number') syncTotal.value = msg.total
      if (typeof msg.processed === 'number') syncProcessed.value = msg.processed
      if (typeof msg.added_count === 'number') syncAddedCount.value = msg.added_count
      return
    }
    if (t === 'sync_node_status') {
      const ev = msg as unknown as GroupSyncNodeEvent & { group_id?: string }
      if (ev.group_id && ev.group_id !== groupId) return
      const next = new Map(syncingNodes.value)
      next.set(ev.node_id, {
        node_id: ev.node_id,
        url: ev.url,
        status: ev.status,
        error: ev.error,
      })
      syncingNodes.value = next
      if (typeof ev.total === 'number') syncTotal.value = ev.total
      if (typeof ev.processed === 'number') syncProcessed.value = ev.processed
      if (typeof ev.added_total === 'number') syncAddedCount.value = ev.added_total
      return
    }
    if (t === 'sync_group_state') {
      const ev = msg as unknown as GroupSyncStateEvent & { group_id?: string }
      if (ev.group_id && ev.group_id !== groupId) return
      isSyncing.value = ev.running
      syncProcessed.value = ev.processed
      syncTotal.value = ev.total
      if (typeof ev.added_count === 'number') syncAddedCount.value = ev.added_count
      if (ev.error) syncError.value = ev.error
      if (ev.synced_at) syncedAt.value = ev.synced_at
      const next = new Map<string, SyncNodeStatus>()
      for (const node of ev.nodes ?? []) {
        next.set(node.node_id, {
          node_id: node.node_id,
          url: node.url,
          status: node.status,
          error: node.error,
        })
      }
      syncingNodes.value = next
      if (!ev.running) {
        maybeUnsubscribe()
      }
      return
    }
    if (t === 'sync_done') {
      const ev = msg as unknown as GroupSyncDoneEvent & { group_id?: string }
      if (ev.group_id && ev.group_id !== groupId) return
      syncedAt.value = ev.synced_at
      if (typeof ev.total === 'number') syncTotal.value = ev.total
      if (typeof ev.processed === 'number') syncProcessed.value = ev.processed
      if (typeof ev.added_count === 'number') syncAddedCount.value = ev.added_count
      isSyncing.value = false
      toast.success('Sync completed', {
        description: `Added ${syncAddedCount.value} nodes`,
      })
      refreshAfterSyncJob()
      maybeUnsubscribe()
      return
    }
    if (t === 'sync_error') {
      const errMsg = typeof msg.error === 'string' ? msg.error : 'sync failed'
      if (msg.group_id && msg.group_id !== groupId) return
      syncError.value = errMsg
      syncTotal.value = typeof msg.total === 'number' ? msg.total : syncTotal.value
      syncProcessed.value = typeof msg.processed === 'number' ? msg.processed : syncProcessed.value
      syncAddedCount.value = typeof msg.added_count === 'number' ? msg.added_count : syncAddedCount.value
      isSyncing.value = false
      toast.error('Sync failed', {
        description: errMsg,
      })
      refreshAfterSyncJob()
      maybeUnsubscribe()
      return
    }
    if (t === 'sync_cancelled') {
      if (msg.group_id && msg.group_id !== groupId) return
      syncTotal.value = typeof msg.total === 'number' ? msg.total : syncTotal.value
      syncProcessed.value = typeof msg.processed === 'number' ? msg.processed : syncProcessed.value
      syncAddedCount.value = typeof msg.added_count === 'number' ? msg.added_count : syncAddedCount.value
      isCancelled.value = true
      isSyncing.value = false
      toast.warning('Sync cancelled')
      refreshAfterSyncJob()
      maybeUnsubscribe()
    }
  }

  function startSync() {
    isSyncing.value = true
    isCancelled.value = false
    syncError.value = ''
    syncProcessed.value = 0
    syncTotal.value = 0
    syncAddedCount.value = 0
    syncedAt.value = ''
    syncingNodes.value = new Map()

    ensureSubscribed()
    sendSSECommand({ action: 'sync_group', group_id: groupId })
  }

  function requestSyncState() {
    ensureSubscribed()
    sendSSECommand({ action: 'sync_group_state', group_id: groupId })
  }

  function stopSync() {
    isSyncing.value = false
    maybeUnsubscribe()
  }

  function cancelSync() {
    if (!isSyncing.value) return
    sendSSECommand({ action: 'cancel_sync', group_id: groupId })
    isCancelled.value = true
    stopSync()
  }

  return {
    isSyncing,
    isCancelled,
    error: syncError,
    syncTotal,
    syncProcessed,
    syncAddedCount,
    syncedAt,
    syncingNodes,
    startSync,
    requestSyncState,
    cancelSync,
    stopSync,
  }
}
