import { z } from 'zod'
import { PublicSourceSchema, CreatePublicSourceSchema, UpdatePublicSourceSchema, type PublicSource, type CreatePublicSource, type UpdatePublicSource } from '~/utils/schemas/public-source'

interface ListPublicSourcesResponse {
  public_sources: unknown[]
}

export async function fetchPublicSources(): Promise<PublicSource[]> {
  const { $api } = useNuxtApp()
  const data = await $api<ListPublicSourcesResponse | unknown[]>('/v1/public-sources')
  const sources = Array.isArray(data) ? data : data.public_sources
  return z.array(PublicSourceSchema).parse(sources)
}

export async function createPublicSource(source: CreatePublicSource): Promise<PublicSource> {
  const { $api } = useNuxtApp()
  const data = await $api<PublicSource>('/v1/public-sources', {
    method: 'POST',
    body: source,
  })
  return PublicSourceSchema.parse(data)
}

export async function updatePublicSource(id: string, source: UpdatePublicSource): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/public-sources/${id}`, {
    method: 'PUT',
    body: source,
  })
}

export async function deletePublicSource(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/public-sources/${id}`, {
    method: 'DELETE',
  })
}

export async function syncPublicSource(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/public-sources/${id}/sync`, {
    method: 'POST',
  })
}
