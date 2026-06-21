import { z } from 'zod'

export const LoginCredentialsSchema = z.object({
  username: z.string().min(1),
  password: z.string().min(1),
})

export const AuthResponseSchema = z.object({
  token: z.string(),
  username: z.string(),
})

export type LoginCredentials = z.infer<typeof LoginCredentialsSchema>
export type AuthResponse = z.infer<typeof AuthResponseSchema>
