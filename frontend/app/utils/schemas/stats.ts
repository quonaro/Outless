import { z } from 'zod'

export const StatsSchema = z.object({
  nodes_total: z.number(),
  tokens_total: z.number(),
  tokens_active: z.number(),
  groups_total: z.number(),
})

export type Stats = z.infer<typeof StatsSchema>
