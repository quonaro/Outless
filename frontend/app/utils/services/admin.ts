import { type ChangeAdminPassword } from '~/utils/schemas/admin'

export async function changeAdminPassword(data: ChangeAdminPassword): Promise<void> {
  const { $api } = useNuxtApp()
  await $api('/v1/admins/change-password', {
    method: 'POST',
    body: data,
  })
}
