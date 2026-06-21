/** Uppercase ISO 3166-1 alpha-2 when input is two letters; otherwise returns trimmed string (legacy rows). */
export function normalizeCountryCode(raw: string): string {
  const s = raw.trim()
  if (s.length !== 2) return s
  const u = s.toUpperCase()
  const a = u.codePointAt(0)!
  const b = u.codePointAt(1)!
  if (a >= 65 && a <= 90 && b >= 65 && b <= 90) return u
  return s
}

/** Unicode regional indicator pair for ISO2; empty if invalid. */
export function countryFlagEmoji(iso2: string): string {
  const c = normalizeCountryCode(iso2)
  if (c.length !== 2) return ''
  const base = 0x1f1e6
  const cpA = base + (c.codePointAt(0)! - 65)
  const cpB = base + (c.codePointAt(1)! - 65)
  return String.fromCodePoint(cpA, cpB)
}

/** Badge label: flag + ISO2, or em dash when unknown. */
export function countryBadgeLabel(iso2: string): string {
  const code = normalizeCountryCode(iso2)
  if (!code || code.length !== 2) return '—'
  const flag = countryFlagEmoji(code)
  return flag ? `${flag} ${code}` : code
}
