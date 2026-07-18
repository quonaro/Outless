import { z } from 'zod'

export const TopUpCheckConfigSchema = z.object({
  workers: z.number().int().nonnegative().optional().default(2),
  timeout: z.string().optional().default('5s'),
  exclude_countries: z.array(z.string()).optional().default([]),
  max_latency: z.string().optional().default(''),
  stages: z.array(z.string()).optional().default(['port', 'handshake']),
})

export const TopUpInputSchema = z.object({
  urls: z.array(z.string()).optional().default([]),
  parser_type: z.string().optional().default('vless_lines'),
  parser_params: z.record(z.any()).optional().default({}),
  check_enabled: z.boolean().optional().default(false),
  check_config: TopUpCheckConfigSchema.optional().default({}),
  schedule_type: z.string().optional().default('interval'),
  schedule_expr: z.string().optional().default(''),
  enabled: z.boolean().optional().default(true),
  next_run_at: z.string().nullable().optional(),
})

export const GroupSchema = z.object({
  id: z.string(),
  name: z.string().min(1),
  total_nodes: z.number().int().nonnegative().optional().default(0),
  random_enabled: z.boolean().optional().default(false),
  random_limit: z.number().int().nonnegative().nullable().optional(),
  is_topup: z.boolean().optional().default(false),
  top_up_id: z.string().optional().default(''),
  created_at: z.string(),
})

export const CreateGroupSchema = z.object({
  name: z.string().min(1),
  random_enabled: z.boolean().optional().default(false),
  random_limit: z.number().int().nonnegative().nullable().optional(),
  top_up: TopUpInputSchema.optional(),
})

export const UpdateGroupSchema = CreateGroupSchema

export type TopUpCheckConfig = z.infer<typeof TopUpCheckConfigSchema>
export type TopUpInput = z.infer<typeof TopUpInputSchema>
export type Group = z.infer<typeof GroupSchema>
export type CreateGroup = z.infer<typeof CreateGroupSchema>
export type UpdateGroup = z.infer<typeof UpdateGroupSchema>
