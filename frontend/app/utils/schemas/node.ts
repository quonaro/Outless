import { z } from 'zod'

export const NodeSchema = z.object({
  id: z.string(),
  url: z.string(),
  group_ids: z.array(z.string()),
  country: z.string(),
  is_self: z.boolean().optional().default(false),
  expires_at: z.string().datetime({ offset: true }).optional(),
})

export const CreateNodeSchema = z.object({
  url: z.string(),
  group_ids: z.array(z.string()).min(1),
  is_self: z.boolean().optional().default(false),
  expires_at: z.string().datetime({ offset: true }).optional(),
})

export const UpdateNodeSchema = z.object({
  url: z.string().min(1).optional(),
  group_ids: z.array(z.string()).optional(),
  expires_at: z.union([z.string().datetime({ offset: true }), z.literal('')]).optional(),
})

export type Node = z.infer<typeof NodeSchema>
export type CreateNode = z.infer<typeof CreateNodeSchema>
export type UpdateNode = z.infer<typeof UpdateNodeSchema>
