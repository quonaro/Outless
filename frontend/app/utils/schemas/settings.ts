import { z } from 'zod'

export const AppLogsSchema = z.object({
  Level: z.string(),
  Colored: z.boolean(),
  Type: z.string(),
  Output: z.string(),
})

export const AppSchema = z.object({
  shutdown_gracetime: z.string(),
  logs: AppLogsSchema,
})

export const GeoIPSchema = z.object({
  db_path: z.string(),
  db_url: z.string(),
  auto: z.boolean(),
  expiry: z.string(),
})

export const DatabaseSchema = z.string()

export const SettingsSchema = z.object({
  database: DatabaseSchema,
  app: AppSchema,
  geoip: GeoIPSchema,
})

export const UpdateSettingsSchema = SettingsSchema

export type AppLogs = z.infer<typeof AppLogsSchema>
export type App = z.infer<typeof AppSchema>
export type GeoIP = z.infer<typeof GeoIPSchema>
export type Settings = z.infer<typeof SettingsSchema>
export type UpdateSettings = z.infer<typeof UpdateSettingsSchema>
