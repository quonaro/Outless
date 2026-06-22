import { z } from 'zod'

export const AppSchema = z.object({
  shutdown_gracetime: z.string(),
  http_port: z.number(),
  log_level: z.string(),
  disable_docs: z.boolean(),
})

export const SettingsSchema = z.object({
  database: z.string(),
  app: AppSchema,
})

export const UpdateSettingsSchema = SettingsSchema

export type App = z.infer<typeof AppSchema>
export type Settings = z.infer<typeof SettingsSchema>
export type UpdateSettings = z.infer<typeof UpdateSettingsSchema>
