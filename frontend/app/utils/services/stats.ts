import {
  EntityTrafficOutputSchema,
  StatsSchema,
  TrafficStatsSchema,
  type EntityTrafficOutput,
  type Stats,
  type TrafficStats,
} from '~/utils/schemas/stats'

export async function fetchStats(): Promise<Stats> {
  const { $api } = useNuxtApp()
  const data = await $api<Stats>('/v1/stats')
  return StatsSchema.parse(data)
}

export async function fetchTrafficStats(): Promise<TrafficStats> {
  const { $api } = useNuxtApp()
  const data = await $api<TrafficStats>('/v1/stats/traffic')
  return TrafficStatsSchema.parse(data)
}

export async function fetchTokenTrafficStats(): Promise<EntityTrafficOutput> {
  const { $api } = useNuxtApp()
  const data = await $api<EntityTrafficOutput>('/v1/stats/traffic/tokens')
  return EntityTrafficOutputSchema.parse(data)
}

export async function fetchNodeTrafficStats(): Promise<EntityTrafficOutput> {
  const { $api } = useNuxtApp()
  const data = await $api<EntityTrafficOutput>('/v1/stats/traffic/nodes')
  return EntityTrafficOutputSchema.parse(data)
}

export async function fetchInboundTrafficStats(): Promise<EntityTrafficOutput> {
  const { $api } = useNuxtApp()
  const data = await $api<EntityTrafficOutput>('/v1/stats/traffic/inbounds')
  return EntityTrafficOutputSchema.parse(data)
}

export async function fetchDomainTrafficStats(): Promise<EntityTrafficOutput> {
  const { $api } = useNuxtApp()
  const data = await $api<EntityTrafficOutput>('/v1/stats/traffic/domains')
  return EntityTrafficOutputSchema.parse(data)
}
