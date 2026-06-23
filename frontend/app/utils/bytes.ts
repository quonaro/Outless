export function formatBytes(v: number): string {
  if (v === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.max(0, Math.floor(Math.log10(v) / 3))
  const unit = units[Math.min(i, units.length - 1)]
  const scaled = v / Math.pow(1000, Math.min(i, units.length - 1))
  return `${scaled.toFixed(2)} ${unit}`
}

export function parseByteSize(value: number, unit: 'MB' | 'GB'): number {
  const multipliers = { MB: 1_000_000, GB: 1_000_000_000 }
  return Math.floor(value * multipliers[unit])
}

export function periodLabel(period: string | undefined): string {
  if (!period) return ''
  const map: Record<string, string> = { day: 'Daily', month: 'Monthly' }
  return map[period] || period
}

export function bytesToSize(bytes: number): { value: number; unit: 'MB' | 'GB' } {
  const gb = bytes / 1_000_000_000
  if (gb >= 1) {
    return { value: Number(gb.toFixed(2)), unit: 'GB' }
  }
  const mb = bytes / 1_000_000
  return { value: Number(mb.toFixed(2)), unit: 'MB' }
}
