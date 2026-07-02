import { ref, onMounted, onUnmounted } from 'vue'

export interface SystemMetricsPoint {
  timestamp: number
  cpu: number
  memory: number
  netRX: number
  netTX: number
  connections: number
}

export interface SystemMetrics {
  cpu_percent: number
  memory_percent: number
  memory_used_bytes: number
  memory_total_bytes: number
  process_memory_used_bytes: number
  net_rx_bytes_per_sec: number
  net_tx_bytes_per_sec: number
  connections_count: number
}

const current = ref<SystemMetrics | null>(null)
const history = ref<SystemMetricsPoint[]>([])
let active = 0
let source: EventSource | null = null

const maxHistory = 60

function onMessage(event: MessageEvent) {
  try {
    const data = JSON.parse(event.data) as SystemMetrics
    current.value = data

    const now = Date.now()
    history.value.push({
      timestamp: now,
      cpu: data.cpu_percent ?? 0,
      memory: data.memory_percent ?? 0,
      netRX: data.net_rx_bytes_per_sec ?? 0,
      netTX: data.net_tx_bytes_per_sec ?? 0,
      connections: data.connections_count ?? 0,
    })
    if (history.value.length > maxHistory) {
      history.value = history.value.slice(-maxHistory)
    }
  } catch {
    // ignore
  }
}

function start() {
  active++
  if (source) return

  const token = useCookie<string | null>('auth_token').value
  const url = token
    ? `/api/v1/stats/system/stream?access_token=${encodeURIComponent(token)}`
    : '/api/v1/stats/system/stream'

  source = new EventSource(url)
  source.onmessage = onMessage
}

function stop() {
  active--
  if (active <= 0 && source) {
    source.close()
    source = null
    active = 0
  }
}

export function useSystemMetrics() {
  onMounted(() => start())
  onUnmounted(() => stop())

  return { current, history }
}
