import { z } from 'zod'

export const NodeSchema = z.object({
  id: z.string(),
  url: z.string(),
  group_id: z.string(),
  country: z.string(),
  is_self: z.boolean().optional().default(false),
})

export const CreateNodeSchema = z.object({
  url: z.string(),
  group_id: z.string().min(1),
  is_self: z.boolean().optional().default(false),
})

export const UpdateNodeSchema = z.object({
  url: z.string().min(1).optional(),
  group_id: z.string().optional(),
})

export type Node = z.infer<typeof NodeSchema>
export type CreateNode = z.infer<typeof CreateNodeSchema>
export type UpdateNode = z.infer<typeof UpdateNodeSchema>
