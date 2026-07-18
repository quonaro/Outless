import { ref, onMounted, onUnmounted, type Ref } from 'vue'

export interface TopUpProgress {
  top_up_id: string
  group_id: string
  group_name?: string
  status: 'idle' | 'pending' | 'running' | 'completed' | 'failed'
  stage: 'idle' | 'pending' | 'fetching' | 'checking' | 'importing'
  total: number
  checked: number
  passed: number
  added: number
  failed: number
  current_url?: string
  error?: string
}

const progress: Ref<Record<string, TopUpProgress>> = ref({})
const isConnected = ref(false)

let eventSource: EventSource | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let subscriberCount = 0

function connect() {
  subscriberCount++
  if (eventSource) return

  const base = typeof window !== 'undefined' ? window.location.origin : ''
  const url = `${base}/api/v1/group-top-ups/stream`

  eventSource = new EventSource(url)
  eventSource.onopen = () => {
    isConnected.value = true
  }
  eventSource.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data) as TopUpProgress
      if (data.group_id) {
        progress.value[data.group_id] = data
      }
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
  }
}

export function useTopUpProgress() {
  onMounted(() => connect())
  onUnmounted(() => disconnect())

  return {
    progress,
    isConnected,
    connect,
    disconnect,
  }
}
