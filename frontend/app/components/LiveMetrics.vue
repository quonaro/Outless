<script setup lang="ts">
import { computed } from 'vue'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  LineElement,
  PointElement,
  CategoryScale,
  LinearScale,
  type TooltipItem,
} from 'chart.js'
import type { SystemMetrics, SystemMetricsPoint } from '~/composables/stats/useSystemMetrics'

ChartJS.register(Title, Tooltip, Legend, LineElement, PointElement, CategoryScale, LinearScale)

const props = defineProps<{
  current: SystemMetrics | null
  history: SystemMetricsPoint[]
}>()

const chartData = computed(() => {
  const labels = props.history.map((p) => {
    const d = new Date(p.timestamp)
    return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`
  })
  return {
    labels,
    datasets: [
      {
        label: 'CPU %',
        data: props.history.map((p) => p.cpu),
        borderColor: 'rgba(99, 102, 241, 1)',
        backgroundColor: 'rgba(99, 102, 241, 0.1)',
        fill: true,
        tension: 0.4,
        pointRadius: 0,
        yAxisID: 'y',
      },
      {
        label: 'Memory %',
        data: props.history.map((p) => p.memory),
        borderColor: 'rgba(16, 185, 129, 1)',
        backgroundColor: 'rgba(16, 185, 129, 0.1)',
        fill: true,
        tension: 0.4,
        pointRadius: 0,
        yAxisID: 'y',
      },
    ],
  }
})

const options = {
  responsive: true,
  maintainAspectRatio: false,
  animation: { duration: 0 },
  interaction: {
    mode: 'index' as const,
    intersect: false,
  },
  plugins: {
    legend: {
      position: 'bottom' as const,
      labels: { usePointStyle: true, boxWidth: 8 },
    },
    tooltip: {
      callbacks: {
        label: (context: TooltipItem<'line'>) =>
          `${context.dataset.label ?? ''}: ${(context.raw as number).toFixed(1)}%`,
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { maxTicksLimit: 6 },
    },
    y: {
      min: 0,
      max: 100,
      grid: { color: 'rgba(128,128,128,0.1)' },
      ticks: { callback: (v: number | string) => `${v}%` },
    },
  },
}
</script>

<template>
  <div class="h-full flex flex-col">
    <div class="flex-1 min-h-0">
      <Line :data="chartData" :options="options" class="h-full w-full" />
    </div>
  </div>
</template>
