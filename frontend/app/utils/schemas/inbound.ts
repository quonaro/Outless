import { z } from 'zod'

export const InboundSchema = z.object({
  id: z.string(),
  name: z.string().min(1),
  address: z.string().default('0.0.0.0'),
  port: z.number().int().default(443),
  sni: z.string().default(''),
  handshake: z.string().default(''),
  public_key: z.string().default(''),
  short_id: z.string().default(''),
  fingerprint: z.string().default('chrome'),
  url_host: z.string().default(''),
  name_template: z.string().default(''),
  enable_auto_self_node: z.boolean().default(false),
  auto_self_node_name: z.string().default('Direct Exit'),
  created_at: z.string(),
  updated_at: z.string(),
})

export const CreateInboundSchema = z.object({
  name: z.string().min(1),
  address: z.string().optional().default('0.0.0.0'),
  port: z.number().int().optional().default(443),
  sni: z.string().optional().default(''),
  handshake: z.string().optional().default(''),
  private_key: z.string().optional().default(''),
  short_id: z.string().optional().default(''),
  fingerprint: z.string().optional().default('chrome'),
  url_host: z.string().optional().default(''),
  name_template: z.string().optional().default(''),
  enable_auto_self_node: z.boolean().optional().default(false),
  auto_self_node_name: z.string().optional().default('Direct Exit'),
})

export const UpdateInboundSchema = CreateInboundSchema

export type Inbound = z.infer<typeof InboundSchema>
export type CreateInbound = z.infer<typeof CreateInboundSchema>
export type UpdateInbound = z.infer<typeof UpdateInboundSchema>
