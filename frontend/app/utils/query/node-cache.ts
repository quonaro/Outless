import type { QueryClient } from '@tanstack/vue-query'
import type { Node } from '~/utils/schemas/node'

type NodePatch = {
  id: string
  country?: string
}

function isNodeArray(value: unknown): value is Node[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'object' && item !== null && 'id' in item)
}

function applyPatch(node: Node, patch: NodePatch): Node {
  if (node.id !== patch.id) return node
  return {
    ...node,
    ...(patch.country != null ? { country: patch.country } : {}),
  }
}

export function patchNodeInCacheData(oldData: unknown, patch: NodePatch): unknown {
  if (isNodeArray(oldData)) {
    return oldData.map((node) => applyPatch(node, patch))
  }
  if (typeof oldData === 'object' && oldData !== null && 'pages' in oldData) {
    const inf = oldData as { pages?: Array<{ nodes?: Node[] }> }
    if (!Array.isArray(inf.pages)) return oldData
    return {
      ...inf,
      pages: inf.pages.map((page) => ({
        ...page,
        nodes: Array.isArray(page.nodes)
          ? page.nodes.map((node) => applyPatch(node, patch))
          : page.nodes,
      })),
    }
  }
  return oldData
}

export function patchNodeInAllNodeQueries(queryClient: QueryClient, patch: NodePatch) {
  queryClient.setQueriesData({ queryKey: ['nodes'] }, (oldData) => patchNodeInCacheData(oldData, patch))
}

function mergeNodePatch(prev: NodePatch | undefined, next: NodePatch): NodePatch {
  if (!prev) return next
  return { ...prev, ...next, id: next.id }
}

function applyPatchesToNode(node: Node, patches: Map<string, NodePatch>): Node {
  const p = patches.get(node.id)
  if (!p) return node
  return applyPatch(node, p)
}

function patchManyInCacheData(oldData: unknown, patches: Map<string, NodePatch>): unknown {
  if (patches.size === 0) return oldData
  if (isNodeArray(oldData)) {
    const seen = new Map<string, Node>()
    for (const node of oldData) {
      if (!seen.has(node.id)) {
        seen.set(node.id, applyPatchesToNode(node, patches))
      }
    }
    return Array.from(seen.values())
  }
  if (typeof oldData === 'object' && oldData !== null && 'pages' in oldData) {
    const inf = oldData as { pages?: Array<{ nodes?: Node[] }> }
    if (!Array.isArray(inf.pages)) return oldData
    return {
      ...inf,
      pages: inf.pages.map((page) => ({
        ...page,
        nodes: Array.isArray(page.nodes)
          ? (() => {
            const seen = new Map<string, Node>()
            for (const node of page.nodes) {
              if (!seen.has(node.id)) {
                seen.set(node.id, applyPatchesToNode(node, patches))
              }
            }
            return Array.from(seen.values())
          })()
          : page.nodes,
      })),
    }
  }
  return oldData
}

let patchBatchRaf: number | null = null
const patchBatchById = new Map<string, NodePatch>()
let patchBatchClient: QueryClient | null = null

function flushPatchBatch() {
  patchBatchRaf = null
  const qc = patchBatchClient
  patchBatchClient = null
  if (!qc || patchBatchById.size === 0) {
    patchBatchById.clear()
    return
  }
  const merged = new Map(patchBatchById)
  patchBatchById.clear()
  qc.setQueriesData({ queryKey: ['nodes'] }, (oldData) => patchManyInCacheData(oldData, merged))
}

/**
 * Coalesces node patches so one TanStack write runs per animation frame
 * (avoids O(nodes × pages) work per WebSocket message and main-thread stalls).
 */
export function schedulePatchNodeInAllNodeQueries(queryClient: QueryClient, patch: NodePatch) {
  patchBatchClient = queryClient
  const prev = patchBatchById.get(patch.id)
  patchBatchById.set(patch.id, mergeNodePatch(prev, patch))
  if (patchBatchRaf != null) return
  patchBatchRaf = requestAnimationFrame(flushPatchBatch)
}

