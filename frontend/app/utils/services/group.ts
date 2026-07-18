import { z } from 'zod'
import {
  GroupSchema,
  CreateGroupSchema,
  UpdateGroupSchema,
  TopUpInputSchema,
  type Group,
  type CreateGroup,
  type UpdateGroup,
  type TopUpInput,
} from '~/utils/schemas/group'

interface ListGroupsResponse {
  groups: unknown[]
}

export async function fetchGroups(): Promise<Group[]> {
  const { $api } = useNuxtApp()
  const data = await $api<ListGroupsResponse | unknown[]>('/v1/groups')
  const groups = Array.isArray(data) ? data : data.groups
  return z.array(GroupSchema).parse(groups)
}

export async function createGroup(group: CreateGroup): Promise<Group> {
  const payload = CreateGroupSchema.parse(group)
  const { $api } = useNuxtApp()
  const data = await $api<Group>('/v1/groups', {
    method: 'POST',
    body: payload,
  })
  return GroupSchema.parse(data)
}

export async function updateGroup(id: string, group: UpdateGroup): Promise<void> {
  const payload = UpdateGroupSchema.parse(group)
  const { $api } = useNuxtApp()
  await $api(`/v1/groups/${id}`, {
    method: 'PUT',
    body: payload,
  })
}

export async function deleteGroup(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/groups/${id}`, {
    method: 'DELETE',
  })
}

interface PingSourceURLResponse {
  ok: boolean
  latency_ms: number
  error?: string
}

export async function pingSourceURL(url: string): Promise<PingSourceURLResponse> {
  const { $api } = useNuxtApp()
  return $api<PingSourceURLResponse>('/v1/group-top-ups/ping', {
    method: 'POST',
    body: { url },
  })
}

export async function fetchTopUp(id: string): Promise<unknown> {
  console.debug('[group.ts] fetchTopUp', id)
  const { $api } = useNuxtApp()
  const response = await $api(`/v1/group-top-ups/${id}`)
  console.debug('[group.ts] fetchTopUp response', response)
  return response
}

export async function runTopUp(id: string): Promise<void> {
  console.debug('[group.ts] runTopUp', id)
  const { $api } = useNuxtApp()
  await $api(`/v1/group-top-ups/${id}/run`, { method: 'POST' })
  console.debug('[group.ts] runTopUp done', id)
}

export interface TopUpRunResult {
  top_up_id: string
  group_id: string
  group_name: string
  status: 'ok' | 'failed'
  total: number
  passed: number
  added: number
  error?: string
}

export async function runAllTopUps(): Promise<TopUpRunResult[]> {
  console.debug('[group.ts] runAllTopUps')
  const { $api } = useNuxtApp()
  const response = await $api<{ results: TopUpRunResult[] }>('/v1/group-top-ups/run', {
    method: 'POST',
  })
  console.debug('[group.ts] runAllTopUps response', response)
  return response.results || []
}

export async function deleteTopUp(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/group-top-ups/${id}`, { method: 'DELETE' })
}

export function buildTopUpInput(values: TopUpFormValues): TopUpInput {
  return TopUpInputSchema.parse({
    urls: values.urlsText
      .split('\n')
      .map((u) => u.trim())
      .filter(Boolean),
    parser_type: values.parserType,
    parser_params: values.parserParams,
    check_enabled: values.checkEnabled,
    check_config: {
      workers: values.workers,
      timeout: values.timeout,
      exclude_countries: values.excludeCountries
        .split(',')
        .map((c) => c.trim())
        .filter(Boolean),
      max_latency: values.maxLatency,
      stages: values.stages,
    },
    schedule_type: values.scheduleType,
    schedule_expr: values.scheduleExpr,
    enabled: values.enabled,
    next_run_at: values.nextRunAt || null,
  })
}

export function defaultTopUpForm(): TopUpFormValues {
  return {
    urlsText: '',
    parserType: 'vless_lines',
    parserParams: {},
    checkEnabled: false,
    workers: 2,
    timeout: '5s',
    excludeCountries: '',
    maxLatency: '',
    stages: ['port', 'handshake'],
    scheduleType: 'interval',
    scheduleExpr: '1h',
    enabled: true,
    nextRunAt: '',
  }
}

export interface TopUpFormValues {
  urlsText: string
  parserType: string
  parserParams: Record<string, unknown>
  checkEnabled: boolean
  workers: number
  timeout: string
  excludeCountries: string
  maxLatency: string
  stages: string[]
  scheduleType: string
  scheduleExpr: string
  enabled: boolean
  nextRunAt: string
}
