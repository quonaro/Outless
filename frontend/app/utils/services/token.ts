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
