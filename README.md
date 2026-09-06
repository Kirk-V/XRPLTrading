# XRPL Trading Tool

A local-first trading application for the XRP Ledger (XRPL). The initial scope is finding and monitoring XRP-to-issued-token markets, viewing live market data and charts, managing wallets and funds, executing trades, and creating/canceling limit offers.

The first read-only XRPL adapter and XRP/token pair-discovery component are now implemented. The implementation is designed for Linux, Windows, and macOS, with testnet support required before any mainnet trading is enabled.

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
- No wallet secrets, signing, or trading code has been enabled yet.

## Run locally

Requirements: Go 1.24 or newer. The current executable is a read-only foundation smoke test.

```bash
go test ./...
go run ./cmd/trader
go build -o bin/trader ./cmd/trader
```

On Windows PowerShell, the build command can be:

```powershell
go build -o bin/trader.exe ./cmd/trader
```

The live XRPL adapter is available to the Go packages but is not yet wired into a user-facing command. The next step is the market ingestion service and first web screen.

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
