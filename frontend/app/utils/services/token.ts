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
