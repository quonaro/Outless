import type { z } from 'zod'
import type { LoginCredentialsSchema} from '~/utils/schemas/auth';
import { AuthResponseSchema } from '~/utils/schemas/auth'

export type LoginCredentials = z.infer<typeof LoginCredentialsSchema>
export type AuthResponse = z.infer<typeof AuthResponseSchema>

export async function login(credentials: LoginCredentials): Promise<AuthResponse> {
  const config = useRuntimeConfig()
  const data = await $fetch(`${config.public.apiBase}/v1/auth/login`, {
    method: 'POST',
    body: credentials,
  })
  return AuthResponseSchema.parse(data)
}
