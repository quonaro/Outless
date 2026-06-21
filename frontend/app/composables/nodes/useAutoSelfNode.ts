import { computed } from 'vue'
import { useInbounds } from '~/composables/inbounds/useInbounds'
import type { Node } from '~/utils/schemas/node'

export interface AutoSelfNode extends Node {
  isAuto: true
  displayName: string
}

export function useAutoSelfNode() {
  const { data: inbounds, isLoading } = useInbounds()

  const activeInbound = computed(() => {
    if (!inbounds.value || inbounds.value.length === 0) return null
    return inbounds.value.find((i) => i.enable_auto_self_node) ?? inbounds.value[0]
  })

  const isEnabled = computed(() => activeInbound.value?.enable_auto_self_node ?? false)
  const nodeName = computed(() => activeInbound.value?.auto_self_node_name || 'Direct Exit')

  // Generate a virtual node for display purposes
  const autoNode = computed<AutoSelfNode | null>(() => {
    if (!activeInbound.value) return null

    return {
      id: '__auto_self_node__',
      url: 'vless://<uuid>@<host>:<port>?...#Direct',
      group_id: '',
      country: 'SELF',
      isAuto: true,
      displayName: nodeName.value,
    }
  })

  // Combine auto node with regular nodes for display
  function combineWithNodes(nodes: Node[]): (Node | AutoSelfNode)[] {
    if (!autoNode.value) return nodes

    // Return auto node first, then regular nodes
    return [autoNode.value, ...nodes]
  }

  return {
    isEnabled,
    nodeName,
    autoNode,
    combineWithNodes,
    isLoading,
  }
}
