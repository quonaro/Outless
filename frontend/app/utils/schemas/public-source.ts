import { z } from 'zod'

export const PublicSourceSchema = z.object({
  id: z.string(),
  url: z.string().url(),
  group_id: z.string(),
  last_fetched_at: z.string().nullable(),
  created_at: z.string(),
})

export const CreatePublicSourceSchema = z.object({
  url: z.string().url(),
  group_id: z.string(),
})

export const UpdatePublicSourceSchema = z.object({
  url: z.string().url(),
  group_id: z.string(),
})

export type PublicSource = z.infer<typeof PublicSourceSchema>
export type CreatePublicSource = z.infer<typeof CreatePublicSourceSchema>
export type UpdatePublicSource = z.infer<typeof UpdatePublicSourceSchema>
