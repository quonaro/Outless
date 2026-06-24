import { z } from 'zod'

export const StatsSchema = z.object({
  nodes_total: z.number(),
  tokens_total: z.number(),
  tokens_active: z.number(),
  groups_total: z.number(),
})

export const TrafficStatsSchema = z.object({
  day_upload_bytes: z.number(),
  day_download_bytes: z.number(),
  month_upload_bytes: z.number(),
  month_download_bytes: z.number(),
})

export const TrafficEntityItemSchema = z.object({
  id: z.string(),
  name: z.string(),
  upload_bytes: z.number(),
  download_bytes: z.number(),
  total_bytes: z.number(),
})

export const EntityTrafficOutputSchema = z.object({
  items: z.array(TrafficEntityItemSchema),
})

export const HistoryDomainItemSchema = TrafficEntityItemSchema

export const HistoryNodeItemSchema = z.object({
  id: z.string(),
  name: z.string(),
  upload_bytes: z.number(),
  download_bytes: z.number(),
  total_bytes: z.number(),
  domains: z.array(HistoryDomainItemSchema),
})

export const HistoryUserItemSchema = z.object({
  id: z.string(),
  name: z.string(),
  upload_bytes: z.number(),
  download_bytes: z.number(),
  total_bytes: z.number(),
  nodes: z.array(HistoryNodeItemSchema),
})

export const DomainHierarchyOutputSchema = z.object({
  items: z.array(HistoryUserItemSchema),
})

export type Stats = z.infer<typeof StatsSchema>
export type TrafficStats = z.infer<typeof TrafficStatsSchema>
export type TrafficEntityItem = z.infer<typeof TrafficEntityItemSchema>
export type EntityTrafficOutput = z.infer<typeof EntityTrafficOutputSchema>
export type HistoryDomainItem = z.infer<typeof HistoryDomainItemSchema>
export type HistoryNodeItem = z.infer<typeof HistoryNodeItemSchema>
export type HistoryUserItem = z.infer<typeof HistoryUserItemSchema>
export type DomainHierarchyOutput = z.infer<typeof DomainHierarchyOutputSchema>
