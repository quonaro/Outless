import { z } from 'zod'

export const GroupSchema = z.object({
  id: z.string(),
  name: z.string().min(1),
  total_nodes: z.number().int().nonnegative().optional().default(0),
  random_enabled: z.boolean().optional().default(false),
  random_limit: z.number().int().nonnegative().nullable().optional(),
  created_at: z.string(),
})

export const CreateGroupSchema = z.object({
  name: z.string().min(1),
  random_enabled: z.boolean().optional().default(false),
  random_limit: z.number().int().nonnegative().nullable().optional(),
})

export const UpdateGroupSchema = CreateGroupSchema

export type Group = z.infer<typeof GroupSchema>
export type CreateGroup = z.infer<typeof CreateGroupSchema>
export type UpdateGroup = z.infer<typeof UpdateGroupSchema>
