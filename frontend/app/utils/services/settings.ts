import { SettingsSchema, type Settings, type UpdateSettings } from '~/utils/schemas/settings'

export async function fetchSettings(): Promise<Settings> {
  const { $api } = useNuxtApp()
  const data = await $api<Settings>('/v1/settings')
  return SettingsSchema.parse(data)
}

export async function updateSettings(settings: UpdateSettings): Promise<void> {
  const { $api } = useNuxtApp()
  await $api('/v1/settings', {
    method: 'PUT',
    body: settings,
  })
}
