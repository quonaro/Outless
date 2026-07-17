export interface ParsedVless {
  host: string
  port: number
  uuid: string
  encryption: string
  flow: string
  network: string
  security: string
  sni: string
  fp: string
  pbk: string
  sid: string
  alpn: string[]
  path: string
  hostHeader: string
  service: string
  spx: string
  name: string
}

export function buildVlessUrl(parsed: ParsedVless): string {
  const url = new URL('vless://example.com')
  url.hostname = parsed.host
  url.port = String(parsed.port)
  url.username = parsed.uuid

  const params = new URLSearchParams()
  if (parsed.encryption && parsed.encryption !== 'none') params.set('encryption', parsed.encryption)
  if (parsed.flow) params.set('flow', parsed.flow)
  if (parsed.network && parsed.network !== 'tcp') params.set('type', parsed.network)
  if (parsed.security && parsed.security !== 'none') params.set('security', parsed.security)
  if (parsed.sni) params.set('sni', parsed.sni)
  if (parsed.fp) params.set('fp', parsed.fp)
  if (parsed.pbk) params.set('pbk', parsed.pbk)
  if (parsed.sid) params.set('sid', parsed.sid)
  if (parsed.path) params.set('path', parsed.path)
  if (parsed.hostHeader) params.set('host', parsed.hostHeader)
  if (parsed.service) params.set('serviceName', parsed.service)
  if (parsed.spx) params.set('spx', parsed.spx)
  if (parsed.alpn.length) params.set('alpn', parsed.alpn.join(','))
  url.search = params.toString()

  if (parsed.name.trim()) url.hash = encodeURIComponent(parsed.name.trim())

  return url.toString()
}

export function updateVlessName(raw: string, name: string): string | null {
  const trimmed = raw.trim()
  if (!trimmed.startsWith('vless://')) return null
  try {
    const parsed = new URL(trimmed)
    if (parsed.protocol !== 'vless:') return null
    parsed.hash = encodeURIComponent(name.trim())
    return parsed.toString()
  } catch {
    return null
  }
}

export function parseVlessUrl(raw: string): ParsedVless | null {
  const trimmed = raw.trim()
  if (!trimmed.startsWith('vless://')) {
    return null
  }

  const urlStr = trimmed
  try {
    const parsed = new URL(urlStr)

    if (parsed.protocol !== 'vless:') {
      return null
    }

    const uuid = parsed.username
    if (!uuid) {
      return null
    }

    const host = parsed.hostname
    const portStr = parsed.port
    const port = portStr ? parseInt(portStr, 10) : 443
    if (Number.isNaN(port)) {
      return null
    }

    const params = parsed.searchParams

    const encryption = params.get('encryption') || 'none'
    const flow = params.get('flow') || ''
    const network = params.get('type') || 'tcp'
    const security = params.get('security') || 'none'
    const sni = params.get('sni') || ''
    const fp = params.get('fp') || ''
    const pbk = params.get('pbk') || ''
    const sid = params.get('sid') || ''
    const path = params.get('path') || ''
    const hostHeader = params.get('host') || ''
    const service = params.get('serviceName') || ''
    const spx = (params.get('spx') || '').trim()
    const name = decodeURIComponent(parsed.hash.replace(/^#/, '')).trim()

    const alpnRaw = params.get('alpn')
    const alpn = alpnRaw
      ? alpnRaw
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean)
      : []

    return {
      host,
      port,
      uuid,
      encryption,
      flow,
      network,
      security,
      sni,
      fp,
      pbk,
      sid,
      alpn,
      path,
      hostHeader,
      service,
      spx,
      name,
    }
  } catch {
    return null
  }
}
