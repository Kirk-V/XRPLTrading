import type { Pair } from './api'

// XRPL's 40-character currency field can contain a readable ASCII label
// followed by zero padding. This is display only; identity stays full code + issuer.
export function currencyLabel(code: string): string {
  if (!/^[A-Fa-f0-9]{40}$/.test(code)) return code
  const bytes = code.match(/../g)!.map((hex) => parseInt(hex, 16))
  const end = bytes.indexOf(0)
  const visible = end < 0 ? bytes : bytes.slice(0, end)
  if (
    visible.length >= 3 &&
    visible.length <= 20 &&
    (end < 0 || bytes.slice(end).every((byte) => byte === 0)) &&
    visible.every((byte) => byte >= 32 && byte <= 126)
  ) {
    return String.fromCharCode(...visible)
  }
  return `${code.slice(0, 8)}…${code.slice(-6)}`
}

export function shortIssuer(issuer: string): string {
  return issuer.length > 15
    ? `${issuer.slice(0, 7)}…${issuer.slice(-6)}`
    : issuer
}

export function matchesPair(pair: Pair, query: string): boolean {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  const currency = pair.quote.currency
  const label = currencyLabel(currency)
  return [
    `XRP/${label}`,
    `XRP / ${label}`,
    label,
    currency,
    pair.quote.issuer ?? '',
  ].some((value) => value.toLowerCase().includes(needle))
}
