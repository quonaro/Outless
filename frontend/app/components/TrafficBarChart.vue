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

ChartJS.register(Title, Tooltip, Legend, BarElement, CategoryScale, LinearScale)

const props = defineProps<{
  dayUpload: number
  dayDownload: number
  monthUpload: number
  monthDownload: number
}>()

function formatBytes(v: number): string {
  if (v === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.max(0, Math.floor(Math.log10(v) / 3))
  const unit = units[Math.min(i, units.length - 1)]
  const scaled = v / Math.pow(1000, Math.min(i, units.length - 1))
  return `${scaled.toFixed(2)} ${unit}`
}

const chartData = computed(() => ({
  labels: ['Today', 'This Month'],
  datasets: [
    {
      label: 'Upload',
      data: [props.dayUpload, props.monthUpload],
      backgroundColor: 'rgba(59, 130, 246, 0.7)',
    },
    {
      label: 'Download',
      data: [props.dayDownload, props.monthDownload],
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
      ticks: {
        callback: (value: number | string) => formatBytes(Number(value)),
      },
    },
  },
}
</script>

<template>
  <div class="h-full flex flex-col">
    <Bar :data="chartData" :options="options" class="flex-1 min-h-0" />
  </div>
</template>
