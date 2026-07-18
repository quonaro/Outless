import { ref, onMounted, onUnmounted } from 'vue'

// Module-level singleton state: only one EventSource for the log stream.
const lines = ref<string[]>([])
const isConnected = ref(false)
let eventSource: EventSource | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let subscriberCount = 0

function connect(maxLines: number) {
  subscriberCount++
  if (eventSource) return

  const base = typeof window !== 'undefined' ? window.location.origin : ''
  eventSource = new EventSource(`${base}/api/v1/events/logs`)
  eventSource.onopen = () => {
    isConnected.value = true
  }
  eventSource.onmessage = (event) => {
    lines.value.push(event.data)
    if (lines.value.length > maxLines) {
      lines.value = lines.value.slice(-maxLines)
    }
  }
  eventSource.onerror = () => {
    isConnected.value = false
    eventSource?.close()
    eventSource = null
    if (subscriberCount > 0) {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        if (subscriberCount > 0) connect(maxLines)
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
    lines.value = []
  }
}

export function useLogStream(maxLines = 200) {
  onMounted(() => connect(maxLines))
  onUnmounted(() => disconnect())

  return { lines, isConnected, connect, disconnect }
}
