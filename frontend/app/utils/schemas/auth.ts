import { z } from 'zod'

export const LoginCredentialsSchema = z.object({
  username: z.string().min(1),
  password: z.string().min(1),
  totp_code: z.string().max(6).optional(),
})

export const AuthResponseSchema = z.object({
  token: z.string().optional(),
  username: z.string().optional(),
  totp_required: z.boolean().default(false),
})

export const TOTPStatusResponseSchema = z.object({
  totp_enabled: z.boolean(),
})

export const TOTPSetupResponseSchema = z.object({
  secret: z.string(),
  uri: z.string(),
  qr_base64: z.string(),
})

export const TOTPVerifySchema = z.object({
  code: z.string().length(6),
})

export const TOTPDisableSchema = z.object({
  code: z.string().length(6),
  password: z.string().min(1),
})

export type LoginCredentials = z.infer<typeof LoginCredentialsSchema>
export type AuthResponse = z.infer<typeof AuthResponseSchema>
export type TOTPStatusResponse = z.infer<typeof TOTPStatusResponseSchema>
export type TOTPSetupResponse = z.infer<typeof TOTPSetupResponseSchema>
export type TOTPVerifyInput = z.infer<typeof TOTPVerifySchema>
export type TOTPDisableInput = z.infer<typeof TOTPDisableSchema>
