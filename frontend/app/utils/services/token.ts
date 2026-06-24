import { z } from 'zod'
import {
  IssuedTokenSchema,
  TokenSchema,
  type CreateToken,
  type IssuedToken,
  type Token,
  type UpdateToken,
} from '~/utils/schemas/token'

interface ListTokensResponse {
  tokens: unknown[]
}

const TrafficItemSchema = z.object({
  period_start: z.string(),
  upload_bytes: z.number(),
  download_bytes: z.number(),
  total_bytes: z.number(),
})

export type TrafficItem = z.infer<typeof TrafficItemSchema>

export async function fetchTokens(): Promise<Token[]> {
  const { $api } = useNuxtApp()
  const data = await $api<ListTokensResponse | unknown[]>('/v1/tokens')
  const tokens = Array.isArray(data) ? data : (data.tokens ?? [])
  return z.array(TokenSchema).parse(tokens)
}

export async function createToken(token: CreateToken): Promise<IssuedToken> {
  const { $api } = useNuxtApp()
  const data = await $api<IssuedToken>('/v1/tokens', {
    method: 'POST',
    body: token,
  })
  return IssuedTokenSchema.parse(data)
}

export async function deactivateToken(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}/deactivate`, {
    method: 'POST',
  })
}

export async function activateToken(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}/activate`, {
    method: 'POST',
  })
}

export async function removeToken(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}`, {
    method: 'DELETE',
  })
}

export async function updateToken(id: string, token: UpdateToken): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}`, {
    method: 'PUT',
    body: token,
  })
}

export async function updateTokenQuota(
  id: string,
  quota: { quota_bytes: number | null; quota_period: string }
): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}/quota`, {
    method: 'PATCH',
    body: quota,
  })
}

export async function fetchTokenTraffic(
  id: string,
  period: 'day' | 'month' = 'day',
  limit = 30
): Promise<TrafficItem[]> {
  const { $api } = useNuxtApp()
  const data = await $api<unknown[]>(`/v1/tokens/${id}/traffic`, {
    query: { period, limit },
  })
  return z.array(TrafficItemSchema).parse(data)
}

export async function resetTokenTraffic(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}/reset-traffic`, {
    method: 'POST',
  })
}

const IPRestrictionSchema = z.object({
  ip: z.string(),
  mode: z.enum(['allow', 'block']),
})

export type IPRestriction = z.infer<typeof IPRestrictionSchema>

export async function fetchTokenIPRestrictions(id: string): Promise<IPRestriction[]> {
  const { $api } = useNuxtApp()
  const data = await $api<unknown[]>(`/v1/tokens/${id}/ips`)
  return z.array(IPRestrictionSchema).parse(data)
}

export async function addTokenIPRestriction(
  id: string,
  ip: string,
  mode: 'allow' | 'block'
): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}/ips`, {
    method: 'POST',
    body: { ip, mode },
  })
}

export async function removeTokenIPRestriction(id: string, ip: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/tokens/${id}/ips/${encodeURIComponent(ip)}`, {
    method: 'DELETE',
  })
}

export async function batchDeactivateTokens(ids: string[]): Promise<void> {
  const { $api } = useNuxtApp()
  await $api('/v1/tokens/batch-deactivate', {
    method: 'POST',
    body: { ids },
  })
}

export async function batchRemoveTokens(ids: string[]): Promise<void> {
  const { $api } = useNuxtApp()
  await $api('/v1/tokens/batch-delete', {
    method: 'POST',
    body: { ids },
  })
}

const ReissueResultSchema = z.object({
  id: z.string(),
  token: z.string(),
  access_url: z.string(),
  owner: z.string(),
})

export type ReissueResult = z.infer<typeof ReissueResultSchema>

export async function reissueToken(id: string): Promise<ReissueResult> {
  const { $api } = useNuxtApp()
  const data = await $api(`/v1/tokens/${id}/reissue`, {
    method: 'POST',
  })
  return ReissueResultSchema.parse(data)
}
