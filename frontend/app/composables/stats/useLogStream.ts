import { ref, onMounted, onUnmounted } from 'vue'

export function useLogStream(maxLines = 200) {
  const lines = ref<string[]>([])
  const isConnected = ref(false)
  let eventSource: EventSource | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  function connect() {
    if (eventSource) return

    const base = typeof window !== 'undefined' ? window.location.origin : ''
    const token = typeof window !== 'undefined' ? localStorage.getItem('auth_token') : null
    const url = token
      ? `${base}/api/v1/events/logs?access_token=${encodeURIComponent(token)}`
      : `${base}/api/v1/events/logs`
    eventSource = new EventSource(url)
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
      reconnectTimer = setTimeout(() => connect(), 3000)
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    eventSource?.close()
    eventSource = null
    isConnected.value = false
  }

  onMounted(() => connect())
  onUnmounted(() => disconnect())

  return { lines, isConnected, connect, disconnect }
}
