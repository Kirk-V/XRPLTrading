import { useState } from 'react'
import { testnetMarkets, type ConnectionState, type MarketSummary } from './api'

function App() {
  const [selected, setSelected] = useState(testnetMarkets[0])
  const [connection, setConnection] = useState<ConnectionState>('connected')

  return (
    <div className="app-shell">
      <header className="topbar">
        <a className="brand" href="/" aria-label="XRPL Trading home">
          <span className="brand-mark" aria-hidden="true">
            X
          </span>
          <span>XRPL Trading</span>
        </a>
        <div className="topbar-actions">
          <span className="network-pill">
            <span className="status-dot" /> TESTNET
          </span>
          <button
            className="secondary-button"
            type="button"
            onClick={() => setConnection('reconnecting')}
          >
            {connection === 'connected' ? 'Read-only mode' : 'Reconnecting…'}
          </button>
          <button
            className="avatar"
            type="button"
            aria-label="Open wallet menu"
          >
            W
          </button>
        </div>
      </header>

      <main className="workspace">
        <aside className="market-sidebar" aria-label="Markets">
          <div className="sidebar-heading">
            <div>
              <p className="eyebrow">MARKETS</p>
              <h1>XRP pairs</h1>
            </div>
            <button
              className="icon-button"
              type="button"
              aria-label="Add market"
            >
              +
            </button>
          </div>
          <label className="search-field">
            <span aria-hidden="true">⌕</span>
            <input
              type="search"
              placeholder="Search markets"
              aria-label="Search markets"
            />
          </label>
          <div className="market-list">
            {testnetMarkets.map((market) => (
              <MarketRow
                key={market.id}
                market={market}
                selected={selected.id === market.id}
                onSelect={setSelected}
              />
            ))}
          </div>
          <div className="discovery-note">
            <span className="sparkle" aria-hidden="true">
              ✦
            </span>
            <div>
              <strong>Pair discovery is on</strong>
              <span>New XRP/token markets will appear here.</span>
            </div>
          </div>
        </aside>

        <section
          className="market-main"
          aria-label={`${selected.symbol} market`}
        >
          <div className="market-header">
            <div>
              <div className="breadcrumb">
                Markets <span>/</span> XRP / {selected.symbol}
              </div>
              <h2>
                XRP <span className="muted-slash">/</span> {selected.symbol}
              </h2>
              <p className="issuer">
                Issuer <code>{selected.issuer}</code>{' '}
                <button
                  type="button"
                  className="copy-button"
                  aria-label="Copy issuer"
                >
                  □
                </button>
              </p>
            </div>
            <div className="price-summary">
              <strong>{selected.quote.mid}</strong>
              <span className="positive">{selected.change24h}</span>
              <small>USD per XRP</small>
            </div>
          </div>

          <div className="stat-row">
            <Stat label="24h activity" value={selected.activity} />
            <Stat label="Spread" value={selected.quote.spread ?? '—'} />
            <Stat
              label="Ledger"
              value={`#${selected.quote.ledger_index.toLocaleString()}`}
            />
            <div className="live-state">
              <span className={`status-dot ${connection}`} />
              {connection === 'connected' ? 'Live' : 'Reconnecting'}
            </div>
          </div>

          <div className="chart-card">
            <div
              className="card-tabs"
              role="tablist"
              aria-label="Chart interval"
            >
              <button className="active" type="button" role="tab">
                Price
              </button>
              <button type="button" role="tab">
                Depth
              </button>
              <button type="button" role="tab">
                Activity
              </button>
              <span className="intervals">
                <button type="button">1H</button>
                <button className="active" type="button">
                  1D
                </button>
                <button type="button">1W</button>
              </span>
            </div>
            <div
              className="chart"
              aria-label="Illustrative XRP USD price chart"
              role="img"
            >
              <div className="chart-grid" />
              <svg
                viewBox="0 0 900 280"
                preserveAspectRatio="none"
                aria-hidden="true"
              >
                <defs>
                  <linearGradient id="fill" x1="0" x2="0" y1="0" y2="1">
                    <stop offset="0" stopColor="#4be0b0" stopOpacity=".24" />
                    <stop offset="1" stopColor="#4be0b0" stopOpacity="0" />
                  </linearGradient>
                </defs>
                <path
                  d="M0 218 C55 211 60 180 115 190 S173 228 218 190 S265 164 306 176 S340 122 390 145 S450 188 493 151 S530 130 568 143 S610 100 645 120 S697 82 734 102 S785 90 820 60 S865 77 900 42 V280 H0Z"
                  fill="url(#fill)"
                />
                <path
                  d="M0 218 C55 211 60 180 115 190 S173 228 218 190 S265 164 306 176 S340 122 390 145 S450 188 493 151 S530 130 568 143 S610 100 645 120 S697 82 734 102 S785 90 820 60 S865 77 900 42"
                  fill="none"
                  stroke="#4be0b0"
                  strokeWidth="3"
                />
              </svg>
              <div className="axis-y">
                <span>0.54</span>
                <span>0.52</span>
                <span>0.50</span>
                <span>0.48</span>
              </div>
              <div className="axis-x">
                <span>00:00</span>
                <span>06:00</span>
                <span>12:00</span>
                <span>18:00</span>
                <span>Now</span>
              </div>
            </div>
            <p className="chart-footnote">
              Testnet data · Last update{' '}
              {new Date(selected.quote.observed_at).toLocaleTimeString([], {
                hour: '2-digit',
                minute: '2-digit',
              })}
            </p>
          </div>

          <div className="lower-grid">
            <OrderBook market={selected} />
            <Activity />
          </div>
        </section>
      </main>
    </div>
  )
}

function MarketRow({
  market,
  selected,
  onSelect,
}: {
  market: MarketSummary
  selected: boolean
  onSelect: (market: MarketSummary) => void
}) {
  return (
    <button
      className={`market-row ${selected ? 'selected' : ''}`}
      type="button"
      onClick={() => onSelect(market)}
    >
      <span>
        <strong>XRP / {market.symbol}</strong>
        <small>{market.issuer}</small>
      </span>
      <span className="row-price">
        <strong>{market.quote.mid}</strong>
        <small className="positive">{market.change24h}</small>
      </span>
    </button>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

function OrderBook({ market }: { market: MarketSummary }) {
  const asks = [
    ['0.5131', '2,410'],
    ['0.5130', '1,820'],
    ['0.5129', '3,104'],
  ]
  const bids = [
    ['0.5124', '4,810'],
    ['0.5123', '2,260'],
    ['0.5121', '5,032'],
  ]
  return (
    <div className="panel">
      <div className="panel-heading">
        <h3>Order book</h3>
        <span>Live · {market.quote.ledger_index.toLocaleString()}</span>
      </div>
      <div className="book-labels">
        <span>PRICE (USD)</span>
        <span>SIZE (XRP)</span>
      </div>
      <div className="book-rows">
        {asks.map(([price, size]) => (
          <div className="book-row ask" key={price}>
            <span>{price}</span>
            <span>{size}</span>
          </div>
        ))}
        <div className="book-mid">
          {market.quote.mid} <span>mid</span>
        </div>
        {bids.map(([price, size]) => (
          <div className="book-row bid" key={price}>
            <span>{price}</span>
            <span>{size}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function Activity() {
  return (
    <div className="panel">
      <div className="panel-heading">
        <h3>Recent activity</h3>
        <span>Validated</span>
      </div>
      <div className="activity-list">
        <div>
          <span className="trade-dot buy" />
          <span>
            <strong>Buy</strong>
            <small>Offer matched · 2 min ago</small>
          </span>
          <b>+1,240 XRP</b>
        </div>
        <div>
          <span className="trade-dot sell" />
          <span>
            <strong>Sell</strong>
            <small>Offer matched · 7 min ago</small>
          </span>
          <b>−680 XRP</b>
        </div>
        <div>
          <span className="trade-dot buy" />
          <span>
            <strong>Buy</strong>
            <small>Offer matched · 11 min ago</small>
          </span>
          <b>+420 XRP</b>
        </div>
      </div>
    </div>
  )
}

export default App
