import { useEffect, useState, type FormEvent } from 'react'
import {
  getJSON,
  pairKey,
  type BookLevel,
  type AMMCreation,
  type MarketState,
  type OrderBook,
  type Pair,
} from './api'

const time = (value: string) => new Date(value).toLocaleTimeString()

function App() {
  const [state, setState] = useState<MarketState>()
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<Pair>()
  const [manual, setManual] = useState<Pair[]>([])
  const [search, setSearch] = useState('')
  const [currency, setCurrency] = useState('')
  const [issuer, setIssuer] = useState('')
  const [formError, setFormError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    let timer: ReturnType<typeof setTimeout>
    let previousNetwork: string | undefined
    async function refresh() {
      try {
        const next = await getJSON<MarketState>(
          '/api/v1/state',
          AbortSignal.any([controller.signal, AbortSignal.timeout(10000)]),
        )
        if (controller.signal.aborted) return
        setState(next)
        setError('')
        if (previousNetwork && previousNetwork !== next.network) {
          setManual([])
          setSelected(next.markets[0])
          setCurrency('')
          setIssuer('')
          setFormError('')
          setSearch('')
        } else {
          setSelected((current) => current ?? next.markets[0])
        }
        previousNetwork = next.network
      } catch (cause) {
        if (!controller.signal.aborted)
          setError(
            cause instanceof Error
              ? cause.message
              : 'Unable to reach the local service.',
          )
      } finally {
        if (!controller.signal.aborted) timer = setTimeout(refresh, 5000)
      }
    }
    void refresh()
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, [])

  const markets = [
    ...new Map(
      [...(state?.markets ?? []), ...manual].map((pair) => [
        pairKey(pair),
        pair,
      ]),
    ).values(),
  ]
  const filtered = markets.filter((pair) =>
    pairKey(pair).toLowerCase().includes(search.toLowerCase()),
  )
  const ammStatus = error
    ? 'offline'
    : (state?.amm_discovery?.connection ?? 'connecting')
  const connection = error ? 'offline' : (state?.connection ?? 'connecting')
  function watch(event: FormEvent) {
    event.preventDefault()
    const code = currency.trim()
    const address = issuer.trim()
    if (
      !/^(?:[A-Za-z0-9?!@#$%^&*<>(){}[\]|]{3}|[A-Fa-f0-9]{40})$/.test(code) ||
      code.toUpperCase() === 'XRP' ||
      !/^r[1-9A-HJ-NP-Za-km-z]{24,34}$/.test(address)
    ) {
      setFormError(
        'Enter a token currency code and its full classic issuer address.',
      )
      return
    }
    const pair: Pair = {
      base: { currency: 'XRP' },
      quote: {
        currency: code.length === 40 ? code.toUpperCase() : code,
        issuer: address,
      },
    }
    setManual((current) =>
      [
        ...current.filter((item) => pairKey(item) !== pairKey(pair)),
        pair,
      ].slice(-200),
    )
    setSelected(pair)
    setFormError('')
  }

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
          <span
            className={`network-pill ${state?.network === 'mainnet' ? 'mainnet' : ''}`}
          >
            {state?.network.toUpperCase() ?? 'CONNECTING'}
          </span>
          <span className="mode-label">Read-only</span>
        </div>
      </header>
      <div className={`connection-banner ${connection}`} role="status">
        <span className={`status-dot ${connection}`} />
        <strong>{connection}</strong>
        <span>
          {error ||
            state?.error ||
            'Validated snapshots · refreshes every 5 seconds'}
        </span>
        {state?.ledger_index ? (
          <span>
            Ledger #{state.ledger_index.toLocaleString()} · closed{' '}
            {time(state.ledger_closed_at)}
          </span>
        ) : null}
      </div>
      <main className="workspace">
        <aside className="market-sidebar" aria-label="Markets">
          <div className="sidebar-heading">
            <div>
              <p className="eyebrow">MARKETS</p>
              <h1>XRP pairs</h1>
            </div>
            <span>{markets.length}</span>
          </div>
          <label className="search-field">
            <input
              type="search"
              placeholder="Search currency or issuer"
              aria-label="Search markets"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <div className="market-list">
            {filtered.map((pair) => (
              <button
                className={`market-row ${selected && pairKey(selected) === pairKey(pair) ? 'selected' : ''}`}
                type="button"
                key={pairKey(pair)}
                onClick={() => setSelected(pair)}
                aria-pressed={!!selected && pairKey(selected) === pairKey(pair)}
              >
                <span>
                  <strong>XRP / {pair.quote.currency}</strong>
                  <small title={pair.quote.issuer}>{pair.quote.issuer}</small>
                </span>
              </button>
            ))}
            {!filtered.length && (
              <p className="empty-state">
                {markets.length
                  ? 'No matching markets.'
                  : 'Waiting for XRP/token activity. You can also watch a market below.'}
              </p>
            )}
          </div>
          <form className="watch-form" onSubmit={watch}>
            <h3>Watch a market</h3>
            <p className="empty-state">
              Updates while selected. This list is not saved after a page
              reload.
            </p>
            <label>
              Currency
              <input
                value={currency}
                onChange={(event) => setCurrency(event.target.value)}
                placeholder="USD or 40-character hex"
                maxLength={40}
                required
              />
            </label>
            <label>
              Issuer
              <input
                value={issuer}
                onChange={(event) => setIssuer(event.target.value)}
                placeholder="Full r… address"
                maxLength={35}
                required
              />
            </label>
            <button className="secondary-button" type="submit">
              Watch market
            </button>
            {formError && (
              <p role="alert" className="error-text">
                {formError}
              </p>
            )}
          </form>
          <section className="amm-discovery" aria-label="AMM discovery">
            <h3>New AMMs</h3>
            <p className="amm-status" role="status">
              <span className={`status-dot ${ammStatus}`} /> Listener:{' '}
              {ammStatus}
            </p>
            {state?.amm_discovery?.error && (
              <p className="error-text">{state.amm_discovery.error}</p>
            )}
            {state?.amm_discovery?.interrupted && (
              <p className="error-text">
                Discovery was interrupted. Pools created during gaps may be
                missing.
              </p>
            )}
            <div className="market-list">
              {(state?.amm_creations ?? []).slice(0, 10).map((creation) => (
                <button
                  className="market-row"
                  type="button"
                  key={creation.transaction_hash}
                  onClick={() => setSelected(creation.pair)}
                  title={`Transaction ${creation.transaction_hash}`}
                >
                  <span>
                    <strong>XRP / {creation.pair.quote.currency}</strong>
                    <small>{creation.pair.quote.issuer}</small>
                    <small>
                      Created · ledger #{creation.ledger_index.toLocaleString()}
                    </small>
                  </span>
                </button>
              ))}
            </div>
            {!state?.amm_creations?.length && (
              <p className="empty-state">
                No new XRP/token pools observed since startup.
              </p>
            )}
            <p className="empty-state">
              Successful, validated creations streamed live. Latest 10 shown;
              existing pools and missed events are not backfilled.
            </p>
          </section>
          <div className="discovery-note">
            <div>
              <strong>Discovering active markets</strong>
              <span>
                Up to 200 pairs from sampled trading activity and new AMMs this
                session. New pools appear even before trading activity.
              </span>
            </div>
          </div>
          {state && <p className="endpoint">Source: {state.endpoint}</p>}
        </aside>
        {selected ? (
          <Market
            key={`${state?.network}:${pairKey(selected)}`}
            pair={selected}
            network={state?.network ?? 'unknown'}
            ammCreation={state?.amm_creations?.find(
              (creation) => pairKey(creation.pair) === pairKey(selected),
            )}
            networkStale={['offline', 'stale', 'reconnecting'].includes(
              connection,
            )}
          />
        ) : (
          <section className="market-main">
            <div className="panel welcome">
              <p className="eyebrow">LIVE MARKET VIEW</p>
              <h2>Your {state?.network ?? 'market'} workspace</h2>
              <p>
                Markets will appear as the service observes activity. Select a
                pair or enter its currency and issuer to view both sides of its
                order book.
              </p>
              <p>
                No wallet is needed. Prices and charts appear only when real
                quotes are available.
              </p>
            </div>
          </section>
        )}
      </main>
    </div>
  )
}

type Point = { value: number; at: string; ledger: number; gap: boolean }
function Market({
  pair,
  network,
  networkStale,
  ammCreation,
}: {
  pair: Pair
  network: string
  networkStale: boolean
  ammCreation?: AMMCreation
}) {
  const [book, setBook] = useState<OrderBook>()
  const [error, setError] = useState('')
  const [points, setPoints] = useState<Point[]>([])
  const [copied, setCopied] = useState(false)
  const [clock, setClock] = useState(Date.now())
  useEffect(() => {
    const controller = new AbortController()
    let timer: ReturnType<typeof setTimeout>
    let gap = false
    const tick = setInterval(() => setClock(Date.now()), 1000)
    async function refresh() {
      try {
        const params = new URLSearchParams({
          currency: pair.quote.currency,
          issuer: pair.quote.issuer!,
        })
        const next = await getJSON<OrderBook>(
          `/api/v1/book?${params}`,
          AbortSignal.any([controller.signal, AbortSignal.timeout(22000)]),
        )
        if (controller.signal.aborted) return
        setBook(next)
        setError('')
        // Only chart coordinates use floating point; displayed prices stay strings.
        const value = Number(next.quote.mid)
        if (next.quote.mid && Number.isFinite(value) && value > 0) {
          const discontinuity = gap
          setPoints((current) =>
            current.at(-1)?.ledger === next.ledger_index
              ? current
              : [
                  ...current,
                  {
                    value,
                    at: next.observed_at,
                    ledger: next.ledger_index,
                    gap: discontinuity,
                  },
                ].slice(-120),
          )
          gap = false
        } else {
          gap = true
        }
      } catch (cause) {
        gap = true
        if (!controller.signal.aborted)
          setError(
            cause instanceof Error
              ? cause.message
              : 'Unable to load the order book.',
          )
      } finally {
        if (!controller.signal.aborted) timer = setTimeout(refresh, 5000)
      }
    }
    void refresh()
    return () => {
      controller.abort()
      clearTimeout(timer)
      clearInterval(tick)
    }
  }, [pair])
  const age = book
    ? Math.max(0, Math.floor((clock - Date.parse(book.observed_at)) / 1000))
    : 0
  const stale = networkStale || !!error || age > 20
  async function copyIssuer() {
    try {
      await navigator.clipboard.writeText(pair.quote.issuer!)
      setCopied(true)
    } catch {
      setCopied(false)
    }
  }
  return (
    <section
      className="market-main"
      aria-label={`${pair.quote.currency} market`}
    >
      <div className="market-header">
        <div>
          <div className="breadcrumb">
            Markets / XRP / {pair.quote.currency}
          </div>
          <h2>
            XRP <span className="muted-slash">/</span> {pair.quote.currency}
          </h2>
          <p className="issuer">
            Issuer <code>{pair.quote.issuer}</code>{' '}
            <button
              className="copy-button"
              type="button"
              onClick={() => void copyIssuer()}
            >
              {copied ? 'Copied' : 'Copy'}
            </button>
          </p>
        </div>
        <div className="price-summary">
          <strong>{book?.quote.mid ?? '—'}</strong>
          <small>{pair.quote.currency} per XRP</small>
        </div>
      </div>
      {ammCreation && (
        <p className="amm-notice">
          AMM creation observed at ledger #
          {ammCreation.ledger_index.toLocaleString()}. This view shows limit
          offers; pool reserves and swap prices are not displayed yet.
        </p>
      )}
      {error && (
        <p role="alert" className="error-banner">
          {error} Retrying automatically.{' '}
          {book ? 'The last successful snapshot is shown below.' : ''}
        </p>
      )}
      <div className="stat-row">
        <Stat label="Best bid" value={book?.quote.bid ?? '—'} />
        <Stat label="Best ask" value={book?.quote.ask ?? '—'} />
        <Stat
          label={`Spread (${pair.quote.currency})`}
          value={book?.quote.spread ?? '—'}
        />
        <div className="live-state" role="status">
          <span
            className={`status-dot ${stale ? 'stale' : book ? 'connected' : 'connecting'}`}
          />
          {stale ? 'Stale' : book ? 'Snapshot' : 'Loading…'}
        </div>
      </div>
      <div className="chart-card">
        <div className="panel-heading">
          <h3>Mid-price · this session</h3>
          <span>{pair.quote.currency} / XRP</span>
        </div>
        <Chart points={points} />
        <p className="chart-footnote">
          Observed quotes, not trade candles or historical data. Up to 120
          samples; gaps are not backfilled.
        </p>
      </div>
      <div className="lower-grid">
        <div className="panel">
          <div className="panel-heading">
            <h3>Order book</h3>
            <span>
              {book
                ? `Ledger #${book.ledger_index.toLocaleString()}`
                : 'Loading…'}
            </span>
          </div>
          <div className="book-labels">
            <span>PRICE ({pair.quote.currency})</span>
            <span>FUNDED SIZE (XRP)</span>
          </div>
          <BookRows rows={book?.asks} side="ask" />
          <div className="book-mid">
            {book?.quote.mid ?? '—'} <span>mid</span>
          </div>
          <BookRows rows={book?.bids} side="bid" />
          <p className="chart-footnote">
            Up to 50 offers per side; showing the best 8. Display values rounded
            to 16 significant digits.
          </p>
        </div>
        <div className="panel">
          <div className="panel-heading">
            <h3>Snapshot details</h3>
          </div>
          <dl className="snapshot-details">
            <dt>Network</dt>
            <dd>{network.toUpperCase()} · read-only</dd>
            <dt>Last fetched</dt>
            <dd>
              {book
                ? `${time(book.observed_at)} (${age}s ago)`
                : 'Waiting for data'}
            </dd>
            <dt>Book offers</dt>
            <dd>
              {book
                ? `${book.bids.length} bids / ${book.asks.length} asks`
                : '—'}
            </dd>
            <dt>Quote</dt>
            <dd>
              {book?.quote.mid
                ? 'Midpoint of best bid and ask'
                : 'A midpoint needs both a bid and an ask.'}
            </dd>
          </dl>
          <p className="empty-state">
            Wallets, trading, and trade history are not connected in this
            read-only view.
          </p>
        </div>
      </div>
    </section>
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
function BookRows({ rows, side }: { rows?: BookLevel[]; side: 'ask' | 'bid' }) {
  if (!rows) return <p className="empty-state">Waiting for {side}s…</p>
  if (!rows.length)
    return <p className="empty-state">No funded {side}s at this ledger.</p>
  const shown = rows.slice(0, 8)
  if (side === 'ask') shown.reverse()
  return (
    <div className="book-rows">
      {shown.map((row, index) => (
        <div
          className={`book-row ${side}`}
          key={`${row.owner}:${row.sequence}:${index}`}
        >
          <span>{row.price}</span>
          <span>{row.size}</span>
        </div>
      ))}
    </div>
  )
}
function Chart({ points }: { points: Point[] }) {
  if (points.length < 2)
    return (
      <div className="chart-empty">
        {points.length
          ? 'First quote received. Waiting for the next validated ledger…'
          : 'Waiting for a two-sided book to start the chart.'}
      </div>
    )
  const low = Math.min(...points.map((point) => point.value))
  const high = Math.max(...points.map((point) => point.value))
  const range = high - low || Math.abs(high) * 0.01 || 1
  const start = Date.parse(points[0].at)
  const duration = Date.parse(points.at(-1)!.at) - start || 1
  const path = points
    .map(
      (point, index) =>
        `${index === 0 || point.gap ? 'M' : 'L'}${20 + ((Date.parse(point.at) - start) / duration) * 860},${240 - ((point.value - low) / range) * 200}`,
    )
    .join(' ')
  return (
    <>
      <svg
        className="session-chart"
        viewBox="0 0 900 280"
        role="img"
        aria-label={`Mid-price chart with ${points.length} observed quotes, from ${low} to ${high}`}
      >
        <path d={path} fill="none" stroke="#4be0b0" strokeWidth="3" />
        <text x="20" y="20" fill="#a9bac9" fontSize="14">
          {high.toPrecision(6)}
        </text>
        <text x="20" y="275" fill="#a9bac9" fontSize="14">
          {low.toPrecision(6)}
        </text>
      </svg>
      <div className="chart-times">
        <span>{time(points[0].at)}</span>
        <span>{time(points.at(-1)!.at)}</span>
      </div>
    </>
  )
}
export default App
