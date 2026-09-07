export type Asset = {
  currency: string
  issuer?: string
}

export type Pair = {
  base: Asset
  quote: Asset
}

export type Quote = {
  pair: Pair
  bid?: string
  ask?: string
  mid?: string
  spread?: string
  ledger_index: number
  observed_at: string
}

export type ConnectionState = 'connected' | 'reconnecting' | 'offline'

export type MarketSummary = {
  id: string
  symbol: string
  issuer: string
  quote: Quote
  change24h: string
  activity: string
}

export const testnetMarkets: MarketSummary[] = [
  {
    id: 'xrp-usd-rTestIssuer',
    symbol: 'USD',
    issuer: 'rTestIssuer…9Kq',
    quote: {
      pair: {
        base: { currency: 'XRP' },
        quote: { currency: 'USD', issuer: 'rTestIssuer' },
      },
      bid: '0.5124',
      ask: '0.5131',
      mid: '0.5128',
      spread: '0.14%',
      ledger_index: 92310421,
      observed_at: new Date().toISOString(),
    },
    change24h: '+2.41%',
    activity: '12.8K XRP',
  },
]
