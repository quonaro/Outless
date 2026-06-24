import { ref, onMounted, onUnmounted } from 'vue'

export interface Connection {
  id: string
  user: string
  node_id: string
  inbound: string
  domain: string
  source_ip: string
  upload: number
  download: number
}

export interface ConnectionsSnapshot {
  upload_total: number
  download_total: number
  connections: Connection[]
}

// Module-level singleton state: only one EventSource per stream type.
const data = ref<ConnectionsSnapshot | null>(null)
const isConnected = ref(false)
let eventSource: EventSource | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let subscriberCount = 0

function connect() {
  subscriberCount++
  if (eventSource) return

  const base = typeof window !== 'undefined' ? window.location.origin : ''
  const token = typeof window !== 'undefined' ? localStorage.getItem('auth_token') : null
  const url = token
    ? `${base}/api/v1/connections/stream?access_token=${encodeURIComponent(token)}`
    : `${base}/api/v1/connections/stream`

  eventSource = new EventSource(url)
  eventSource.onopen = () => {
    isConnected.value = true
  }
  eventSource.onmessage = (event) => {
    try {
      const parsed = JSON.parse(event.data) as ConnectionsSnapshot
      data.value = parsed
    } catch {
      // ignore malformed events
    }
  }
  eventSource.onerror = () => {
    isConnected.value = false
    eventSource?.close()
    eventSource = null
    if (subscriberCount > 0) {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        if (subscriberCount > 0) connect()
      }, 3000)
    }
  }
}

function disconnect() {
  subscriberCount--
  if (subscriberCount <= 0) {
    subscriberCount = 0
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    eventSource?.close()
    eventSource = null
    isConnected.value = false
    data.value = null
  }
}

export function useConnectionsStream() {
  onMounted(() => {
    connect()
  })

  onUnmounted(() => {
    disconnect()
  })

  return {
    data,
    isConnected,
    connect,
    disconnect,
  }
}
