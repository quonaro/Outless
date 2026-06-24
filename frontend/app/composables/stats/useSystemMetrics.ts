import { ref, onMounted, onUnmounted } from 'vue'
import { getAuthHeaders } from '~/utils/services/auth-header'

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
  net_rx_bytes_per_sec: number
  net_tx_bytes_per_sec: number
  connections_count: number
}

const current = ref<SystemMetrics | null>(null)
const history = ref<SystemMetricsPoint[]>([])
let timer: ReturnType<typeof setInterval> | null = null
let active = 0

const maxHistory = 60

async function fetchMetrics(): Promise<void> {
  try {
    const res = await fetch('/api/v1/stats/system', {
      headers: getAuthHeaders(),
    })
    if (!res.ok) return
    const data = (await res.json()) as SystemMetrics
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
  if (timer) return
  fetchMetrics()
  timer = setInterval(fetchMetrics, 5000)
}

function stop() {
  active--
  if (active <= 0 && timer) {
    clearInterval(timer)
    timer = null
    active = 0
  }
}

export function useSystemMetrics() {
  onMounted(() => start())
  onUnmounted(() => stop())

  return { current, history, fetchMetrics }
}
