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
