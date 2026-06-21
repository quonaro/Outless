import { z } from 'zod'
import {
  InboundSchema,
  CreateInboundSchema,
  UpdateInboundSchema,
  type Inbound,
  type CreateInbound,
  type UpdateInbound,
} from '~/utils/schemas/inbound'

interface ListInboundsResponse {
  inbounds: unknown[]
}

export async function fetchInbounds(): Promise<Inbound[]> {
  const { $api } = useNuxtApp()
  const data = await $api<ListInboundsResponse | unknown[]>('/v1/inbounds')
  const inbounds = Array.isArray(data) ? data : data.inbounds
  return z.array(InboundSchema).parse(inbounds)
}

export async function createInbound(inbound: CreateInbound): Promise<Inbound> {
  const payload = CreateInboundSchema.parse(inbound)
  const { $api } = useNuxtApp()
  const data = await $api<Inbound>('/v1/inbounds', {
    method: 'POST',
    body: payload,
  })
  return InboundSchema.parse(data)
}

export async function updateInbound(id: string, inbound: UpdateInbound): Promise<void> {
  const payload = UpdateInboundSchema.parse(inbound)
  const { $api } = useNuxtApp()
  await $api(`/v1/inbounds/${id}`, {
    method: 'PUT',
    body: payload,
  })
}

export async function deleteInbound(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/inbounds/${id}`, {
    method: 'DELETE',
  })
}

export async function generateKeypair(): Promise<{ private_key: string; public_key: string }> {
  const { $api } = useNuxtApp()
  return await $api('/v1/inbounds/keypair')
}
