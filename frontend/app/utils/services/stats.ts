import { StatsSchema, type Stats } from '~/utils/schemas/stats'

export async function fetchStats(): Promise<Stats> {
  const { $api } = useNuxtApp()
  const data = await $api<Stats>('/v1/stats')
  return StatsSchema.parse(data)
}
