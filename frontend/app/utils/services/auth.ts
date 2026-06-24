import type { z } from 'zod'
import {
  AuthResponseSchema,
  TOTPStatusResponseSchema,
  TOTPSetupResponseSchema,
  type LoginCredentialsSchema,
  type TOTPVerifyInput,
  type TOTPDisableInput,
} from '~/utils/schemas/auth'

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

export async function getTOTPStatus(): Promise<z.infer<typeof TOTPStatusResponseSchema>> {
  const { $api } = useNuxtApp()
  const data = await $api('/v1/auth/totp/status', {
    method: 'GET',
  })
  return TOTPStatusResponseSchema.parse(data)
}

export async function setupTOTP(): Promise<z.infer<typeof TOTPSetupResponseSchema>> {
  const { $api } = useNuxtApp()
  const data = await $api('/v1/auth/totp/setup', {
    method: 'POST',
  })
  return TOTPSetupResponseSchema.parse(data)
}

export async function verifyTOTP(input: TOTPVerifyInput): Promise<void> {
  const { $api } = useNuxtApp()
  await $api('/v1/auth/totp/verify', {
    method: 'POST',
    body: input,
  })
}

export async function disableTOTP(input: TOTPDisableInput): Promise<void> {
  const { $api } = useNuxtApp()
  await $api('/v1/auth/totp/disable', {
    method: 'POST',
    body: input,
  })
}
