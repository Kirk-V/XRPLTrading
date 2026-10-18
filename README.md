# XRPL Trading Tool

A local-first trading application for the XRP Ledger (XRPL). The initial scope is finding and monitoring XRP-to-issued-token markets, viewing live market data and charts, managing wallets and funds, executing trades, and creating/canceling limit offers.

The read-only XRPL adapter, market ingestion foundation, durable event journal, versioned API models, and a connected read-only web dashboard are implemented. The implementation is designed for Linux, Windows, and macOS, with testnet support required before any mainnet trading is enabled.

## Product direction

The recommended product is a native desktop application with a web-based UI:

- Go service for XRPL connectivity, market data, wallet operations, signing, persistence, and domain rules.
- React + TypeScript UI for a sleek, responsive, accessible experience.
- A small desktop shell (Wails is the current candidate) for distribution, native secure-storage integration, and a single-app experience.
- Browser development mode that talks to the local Go service.

This gives us the benefits of web UI development without requiring a hosted backend or sending private keys to a website.

## Current status

- [Implementation plan](PLAN.md)
- [Architecture overview](ARCHITECTURE.md)
- Go foundation, safe testnet defaults, and domain validation are implemented.
- JSON-RPC connectivity, `book_offers`, and WebSocket book subscriptions are implemented.
- XRP/token pair discovery and direction deduplication are implemented.
- A reconnecting market ingestion service now reloads a book snapshot after stream interruption.
- A reconnecting validated-ledger stream is available for market cursors and future backfill.
- Normalized market events can be durably journaled and replayed from local JSONL storage.
- Versioned Go API read models cover quotes, candles, wallets, balances, orders, and transactions.
- A Vite React/TypeScript dashboard connects to a local Go HTTP API for testnet or read-only mainnet market discovery, validated order-book snapshots, and a session mid-price chart.
- No wallet secrets, signing, or trading code has been enabled yet.

## Run locally

Requirements: Go 1.24 or newer and Node.js 22.13 or newer (Node 24 LTS also works).

From the repository root, build the UI once and start the app:

```bash
npm --prefix web ci
npm --prefix web run build
go run ./cmd/trader
```

To view the real network, start the service with:

```bash
go run ./cmd/trader -network mainnet
```

Mainnet uses Ripple's public `s1.ripple.com` HTTP/WebSocket endpoints and remains read-only. Market discovery, AMM detection, and order-book queries use the selected network. The UI displays a prominent **MAINNET** label. Omitting `-network` still defaults to testnet; use `-network testnet` to select it explicitly. Stop the existing service before switching. Reloading clears manual watches; an open tab also clears its selected market and chart when it detects a network change.

Open **http://localhost:8080**. The Go process serves both the built UI and its read-only API. Stop it with Ctrl+C. Use `-port 8081` if port 8080 is occupied; `-web /path/to/web/dist` selects a different built UI directory.

For UI development, keep the Go service running on port 8080, then in another terminal:

```bash
npm --prefix web run dev
```

Open the Vite URL printed in that terminal. Vite proxies `/api` to the local Go service. Frontend changes hot-reload; production UI changes require another `npm --prefix web run build`.

Validation and a standalone Go build:

```bash
go test -race ./api/... ./internal/... ./cmd/...
npm --prefix web run build
npm --prefix web run lint
npm --prefix web run format
go build -o bin/trader ./cmd/trader
```

On Windows, use `go build -o bin/trader.exe ./cmd/trader`. Run from the repository root or pass `-web` so the binary can find the built UI.

## How market discovery works

Discovery runs on the network selected at startup: testnet by default, or mainnet with `-network mainnet`. Both modes are read-only and support XRP/issued-token pairs; token/token markets are not included.

| Source | How it works | What it finds |
| --- | --- | --- |
| Trading activity | The Go service calls `book_changes` for the latest validated ledger, waiting five seconds after each request before polling again. | XRP/token pairs with activity in the sampled ledgers. |
| New AMMs | A continuous WebSocket subscription listens to transactions and ledger heartbeats. Only validated `AMMCreate` transactions whose metadata reports `tesSUCCESS` are accepted. | Newly created XRP/token pools, without waiting for later trading activity. |

Tokens are identified by their **currency code and full issuer address**, not their ticker alone. Pairs are normalized with XRP as the base asset, so observations in opposite directions do not create duplicates. An AMM creation is still reported when its pair was already discovered through trading activity.

The server retains up to **200 discovered pairs** in memory. Trading-activity discovery stops adding pairs when this limit is reached; a new AMM can still add its pair by evicting the oldest retained pair. The server also keeps the latest **100 AMM creation events**, deduplicated by transaction hash within that retained history; the **New AMMs** panel shows the latest **10**. Discovered pairs and creation history reset when the Go service restarts. The browser sees updates through its five-second API polling cycle.

### Discovery limits and connection status

- This is not a complete catalog of existing markets or pools. Quiet markets may not appear through activity discovery, and pools created before the listener starts are not enumerated.
- Activity polling can skip intermediate ledgers. The AMM listener reconnects automatically, but events missed during disconnects are not backfilled.
- The AMM listener has a separate status from market polling. It uses ledger heartbeats to detect a silent connection and flags interruptions or ledger gaps; that warning remains for the server session after reconnection.
- A new pool may have no limit offers. The current book view does not display AMM reserves or swap quotes.
- Discovery does not assess a token's quality or liquidity and never places trades.

### Tracking a specific market

Select a discovered pair, search by currency/issuer, or enter the exact currency code and full issuer address under **Watch a market**. Manual selection works even when discovery has not observed the pair. No wallet is required.

Only the selected market refreshes, about every five seconds after each completed request. Both sides of its order book use the same validated ledger; prices are token units per XRP, and sizes use funded XRP quantities. Empty books are shown explicitly. Failed refreshes retain the last snapshot with a stale warning and retry automatically.

Manual watch entries are stored only in the browser session and disappear on page reload. Unselected markets are not monitored in the background. The chart holds up to 120 midpoint observations for the current selection, requires both bids and asks, and resets when switching markets, reloading, or changing networks. It is not historical candles or a trade feed.

A saved watchlist, background market collection, historical backfill, journal-backed charts, AMM pricing, and trading remain follow-up work. The current dashboard does not use the existing book ingestor or durable event journal. See [ARCHITECTURE.md](ARCHITECTURE.md#connected-read-only-dashboard) for implementation details and [PLAN.md](PLAN.md#next-session-handoff) for the development handoff.

## Safety principles

Never commit seeds, private keys, or unencrypted wallet exports. Transactions must be signed locally and submitted as signed blobs. The application must display network, destination, asset issuer, amount, estimated fee, slippage/price limits, and final transaction hash before considering an operation complete.

Use testnet fixtures and simulated execution until the transaction lifecycle, reconnection behavior, and recovery paths have been tested thoroughly.

## Proposed repository layout

```text
cmd/trader/              Go application entry point
internal/                private Go services and adapters
  xrpl/                  XRPL client, subscriptions, ledger and transaction adapters
  market/                pair discovery, books, trades, candles and normalization
  wallet/                wallet metadata, encrypted secret handling and signing
  order/                 offer construction, validation and lifecycle tracking
  storage/               migrations and persistence
api/                     versioned API contracts
web/                     React + TypeScript frontend
desktop/                 native desktop packaging/integration
tests/                   integration, replay and end-to-end fixtures
docs/                    ADRs and operational/security documentation
```

## Development roadmap

See [PLAN.md](PLAN.md). The first executable milestone is a read-only testnet vertical slice: connect, discover XRP/token pairs, stream order-book updates, and render a chart. Trading is deliberately a later milestone.

## XRPL references

- [XRPL Offers](https://xrpl.org/docs/concepts/tokens/decentralized-exchange/offers)
- [XRPL subscribe method](https://xrpl.org/docs/references/http-websocket-apis/public-api-methods/subscription-methods/subscribe)
- [XRPL secure signing](https://xrpl.org/docs/concepts/transactions/secure-signing)
- [XRPL submit method](https://xrpl.org/docs/references/http-websocket-apis/public-api-methods/transaction-methods/submit)
