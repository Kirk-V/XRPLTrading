export type Asset = { currency: string; issuer?: string }
export type Pair = { base: Asset; quote: Asset }
export type Quote = {
  pair: Pair
  bid?: string
  ask?: string
  mid?: string
  spread?: string
  ledger_index: number
  observed_at: string
}
export type ConnectionState =
  'connecting' | 'connected' | 'reconnecting' | 'stale'
export type MarketState = {
  network: string
  read_only: boolean
  endpoint: string
  connection: ConnectionState
  error?: string
  ledger_index: number
  ledger_closed_at: string
  observed_at: string
  markets: Pair[]
  amm_discovery: {
    connection: ConnectionState
    error?: string
    ledger_index: number
    last_message_at: string
    interrupted: boolean
  }
  amm_creations: AMMCreation[]
  amm_statuses: AMMStatus[]
}
export type AMMCreation = {
  pair: Pair
  ledger_index: number
  transaction_hash: string
  observed_at: string
}
export type AMMStatus = {
  pair: Pair
  state: 'active' | 'inactive' | 'none' | 'unknown'
  ledger_index?: number
  checked_at?: string
}
export type BookLevel = {
  price: string
  size: string
  owner: string
  sequence: number
}
export type OrderBook = {
  pair: Pair
  quote: Quote
  bids: BookLevel[]
  asks: BookLevel[]
  ledger_index: number
  observed_at: string
}
export const pairKey = (pair: Pair) =>
  `${pair.quote.currency}:${pair.quote.issuer}`

export async function getJSON<T>(
  path: string,
  signal: AbortSignal,
): Promise<T> {
  const response = await fetch(path, { signal, cache: 'no-store' })
  if (!response.headers.get('content-type')?.includes('application/json')) {
    throw new Error(
      'Local service unavailable. Start it with go run ./cmd/trader.',
    )
  }
  const data = await response.json()
  if (!response.ok)
    throw new Error(data.error || `Request failed (${response.status})`)
  return data as T
}
