import { z } from 'zod'
import { GroupSchema, CreateGroupSchema, UpdateGroupSchema, type Group, type CreateGroup, type UpdateGroup } from '~/utils/schemas/group'

interface ListGroupsResponse {
  groups: unknown[]
}

export async function fetchGroups(): Promise<Group[]> {
  const { $api } = useNuxtApp()
  const data = await $api<ListGroupsResponse | unknown[]>('/v1/groups')
  const groups = Array.isArray(data) ? data : data.groups
  return z.array(GroupSchema).parse(groups)
}

export async function createGroup(group: CreateGroup): Promise<Group> {
  const payload = CreateGroupSchema.parse(group)
  const { $api } = useNuxtApp()
  const data = await $api<Group>('/v1/groups', {
    method: 'POST',
    body: payload,
  })
  return GroupSchema.parse(data)
}

export async function updateGroup(id: string, group: UpdateGroup): Promise<void> {
  const payload = UpdateGroupSchema.parse(group)
  const { $api } = useNuxtApp()
  await $api(`/v1/groups/${id}`, {
    method: 'PUT',
    body: payload,
  })
}

export async function deleteGroup(id: string): Promise<void> {
  const { $api } = useNuxtApp()
  await $api(`/v1/groups/${id}`, {
    method: 'DELETE',
  })
}

export interface GroupSyncNodeEvent {
  group_id?: string
  node_id: string
  url: string
  status: 'importing' | 'done' | 'error'
  processed?: number
  total?: number
  added_total?: number
  error?: string
}

export interface GroupSyncDoneEvent {
  synced_at: string
  processed?: number
  total?: number
  added_count?: number
  group_id?: string
}

export interface GroupSyncStateEvent {
  group_id?: string
  running: boolean
  processed: number
  total: number
  nodes: GroupSyncNodeEvent[]
  error?: string
  synced_at?: string
  added_count?: number
}

