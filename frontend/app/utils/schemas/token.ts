import { z } from 'zod'

export const TokenSchema = z.object({
  id: z.string(),
  owner: z.string(),
  group_id: z.string(),
  group_ids: z.array(z.string()).optional().default([]),
  access_url: z.string().optional().default(''),
  is_active: z.boolean(),
  expires_at: z.string(),
  created_at: z.string(),
})

export const CreateTokenSchema = z.object({
  owner: z.string().min(1),
  group_ids: z.array(z.string()).optional().default([]),
  expires_in: z.string().min(1),
})

export const UpdateTokenSchema = z.object({
  owner: z.string().min(1),
  group_ids: z.array(z.string()).optional().default([]),
  expires_in: z.string().min(1),
})

export const IssuedTokenSchema = TokenSchema.extend({
  token: z.string(),
})

export type Token = z.infer<typeof TokenSchema>
export type CreateToken = z.infer<typeof CreateTokenSchema>
export type UpdateToken = z.infer<typeof UpdateTokenSchema>
export type IssuedToken = z.infer<typeof IssuedTokenSchema>

// ExpiresInOption lists predefined durations shown in the UI.
export interface ExpiresInOption {
  label: string
  value: string
}

export const EXPIRES_IN_OPTIONS: ExpiresInOption[] = [
  { label: '1 day', value: '24h' },
  { label: '7 days', value: '168h' },
  { label: '30 days', value: '720h' },
  { label: '90 days', value: '2160h' },
  { label: '1 year', value: '8760h' },
]
