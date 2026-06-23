<script setup lang="ts">
import { computed } from 'vue'
import { Bar } from 'vue-chartjs'
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  BarElement,
  CategoryScale,
  LinearScale,
  type TooltipItem,
} from 'chart.js'
import type { TrafficItem } from '~/utils/services/token'

ChartJS.register(Title, Tooltip, Legend, BarElement, CategoryScale, LinearScale)

const props = defineProps<{
  items: TrafficItem[]
}>()

function formatBytes(v: number): string {
  if (v === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.max(0, Math.floor(Math.log10(v) / 3))
  const unit = units[Math.min(i, units.length - 1)]
  const scaled = v / Math.pow(1000, Math.min(i, units.length - 1))
  return `${scaled.toFixed(2)} ${unit}`
}

function shortDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

const sorted = computed(() => {
  const copy = [...props.items]
  copy.sort((a, b) => a.period_start.localeCompare(b.period_start))
  return copy
})

const chartData = computed(() => ({
  labels: sorted.value.map((i) => shortDate(i.period_start)),
  datasets: [
    {
      label: 'Upload',
      data: sorted.value.map((i) => i.upload_bytes),
      backgroundColor: 'rgba(59, 130, 246, 0.7)',
    },
    {
      label: 'Download',
      data: sorted.value.map((i) => i.download_bytes),
      backgroundColor: 'rgba(16, 185, 129, 0.7)',
    },
  ],
}))

const options = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { position: 'bottom' as const },
    tooltip: {
      callbacks: {
        label: (context: TooltipItem<'bar'>) =>
          `${context.dataset.label ?? ''}: ${formatBytes(context.raw as number)}`,
      },
    },
  },
  scales: {
    y: {
      stacked: true,
      ticks: {
        callback: (value: number | string) => formatBytes(Number(value)),
      },
    },
    x: {
      stacked: true,
    },
  },
}
</script>

<template>
  <div class="h-64 w-full">
    <Bar :data="chartData" :options="options" />
  </div>
</template>
