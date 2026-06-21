import type { QueryClient } from '@tanstack/vue-query'
import { computed, readonly, ref } from 'vue'
import { useAuth } from '~/composables/useAuth'

let queryClient: QueryClient | null = null
let apiBase = ''
let getToken: () => string | null = () => null

let abortController: AbortController | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
const sseConnected = ref(false)
const sseConnecting = ref(false)
const sseEverOpened = ref(false)

const globalSubs = new Set<(msg: Record<string, unknown>) => void>()
const groupSyncSubs = new Map<string, Set<(msg: Record<string, unknown>) => void>>()
const openHandlers = new Set<() => void>()

export function setSSEQueryClient(q: QueryClient) {
  queryClient = q
}

export function setSSEConfig(baseURL: string, tokenGetter: () => string | null) {
  apiBase = baseURL.replace(/\/$/, '')
  getToken = tokenGetter
}

function buildSSEURL(base: string): string {
  let u: URL
  if (base.startsWith('http://') || base.startsWith('https://')) {
    u = new URL(base)
  }
  else {
    const path = base.startsWith('/') ? base.slice(1) : base
    u = new URL(path || 'api', typeof window !== 'undefined' ? window.location.origin : 'http://127.0.0.1')
  }
  const pathPrefix = u.pathname === '/' ? '' : u.pathname.replace(/\/$/, '')
  return `${u.protocol}//${u.host}${pathPrefix}/v1/events`
}

function dispatch(msg: Record<string, unknown>) {
  const t = msg.type
  if (t === 'invalidate' && queryClient) {
    const keys = msg.keys
    if (Array.isArray(keys)) {
      for (const k of keys) {
        if (typeof k !== 'string') continue
        if (k === 'nodes') {
          void queryClient.invalidateQueries({ queryKey: ['nodes', 'infinite'] })
          continue
        }
        void queryClient.invalidateQueries({ queryKey: [k] })
      }
    }
    return
  }
  for (const cb of [...globalSubs]) cb(msg)
  const gid = typeof msg.group_id === 'string' ? msg.group_id : ''
  if (!gid) return
  const set = groupSyncSubs.get(gid)
  if (!set) return
  for (const cb of [...set]) cb(msg)
}

function notifyOpen() {
  for (const h of [...openHandlers]) h()
}

export function subscribeSSE(cb: (msg: Record<string, unknown>) => void): () => void {
  globalSubs.add(cb)
  return () => globalSubs.delete(cb)
}

export function onSSEOpen(handler: () => void): () => void {
  openHandlers.add(handler)
  return () => openHandlers.delete(handler)
}

export function subscribeGroupSyncChannel(groupId: string, cb: (msg: Record<string, unknown>) => void): () => void {
  if (!groupSyncSubs.has(groupId)) groupSyncSubs.set(groupId, new Set())
  const s = groupSyncSubs.get(groupId)!
  s.add(cb)
  return () => {
    s.delete(cb)
    if (s.size === 0) groupSyncSubs.delete(groupId)
  }
}

async function sendCommand(path: string, body?: object) {
  const token = getToken()
  if (!token) throw new Error('not authenticated')
  const base = apiBase.startsWith('http') ? apiBase : `${window.location.origin}${apiBase}`
  const url = `${base.replace(/\/$/, '')}${path}`
  const res = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`)
  }
}

export function sendSSECommand(payload: { action: string; group_id: string }) {
  const { action, group_id } = payload
  if (action === 'sync_group') {
    return sendCommand(`/v1/groups/${group_id}/sync`)
  }
  if (action === 'cancel_sync') {
    return sendCommand(`/v1/groups/${group_id}/sync/cancel`)
  }
  if (action === 'sync_group_state') {
    // No-op: state is pushed automatically by SSE
    return Promise.resolve()
  }
  return Promise.reject(new Error(`unknown action: ${action}`))
}

export function disconnectSSE() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (abortController) {
    abortController.abort()
    abortController = null
  }
  sseConnected.value = false
  sseConnecting.value = false
}

function handle401() {
  const auth = useAuth()
  auth.clearToken()
  if (typeof window !== 'undefined') {
    window.location.href = '/login'
  }
}

export function connectSSE() {
  if (typeof window === 'undefined') return
  const token = getToken()
  if (!token || !apiBase) {
    disconnectSSE()
    return
  }

  if (sseConnected.value || sseConnecting.value) return

  disconnectSSE()
  sseConnecting.value = true

  const url = buildSSEURL(apiBase)
  abortController = new AbortController()

  fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
    signal: abortController.signal,
  }).then(async (res) => {
    if (res.status === 401) {
      handle401()
      return
    }
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    sseConnected.value = true
    sseConnecting.value = false
    if (!sseEverOpened.value) {
      sseEverOpened.value = true
    }
    notifyOpen()

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      let currentData = ''
      for (const line of lines) {
        const trimmed = line.trim()
        if (trimmed === '') {
          if (currentData) {
            try {
              const msg = JSON.parse(currentData)
              dispatch(msg)
            }
            catch {
              // ignore malformed
            }
            currentData = ''
          }
          continue
        }
        if (trimmed.startsWith('data: ')) {
          currentData = trimmed.slice(6)
        }
      }
    }
  }).catch((err) => {
    if (err.name === 'AbortError') return
    sseConnected.value = false
    sseConnecting.value = false
    const t = getToken()
    if (t && apiBase) {
      reconnectTimer = setTimeout(() => connectSSE(), 3000)
    }
  }).finally(() => {
    sseConnected.value = false
    sseConnecting.value = false
    if (getToken() && apiBase) {
      reconnectTimer = setTimeout(() => connectSSE(), 3000)
    }
  })
}

export function ensureSSEConnected() {
  const token = getToken()
  if (!token) return
  if (sseConnected.value || sseConnecting.value) return
  connectSSE()
}

export function useSSEStatus() {
  const isBackendAvailable = computed(() => sseConnected.value)
  return {
    isConnected: readonly(sseConnected),
    isConnecting: readonly(sseConnecting),
    isBackendAvailable,
  }
}
