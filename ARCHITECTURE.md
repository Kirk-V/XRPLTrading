# XRPL Trading Tool — Architecture Overview

## System shape

```text
┌──────────────────────────────────────────────────────────────┐
│ Desktop shell (Go + web runtime)                             │
│  React/TypeScript UI                                         │
│  pair search · charts · wallet screens · order review        │
└───────────────┬──────────────────────────────────────────────┘
                │ local typed API / event stream
┌───────────────▼──────────────────────────────────────────────┐
│ Go application service                                       │
│  auth/session boundary · command handlers · event fan-out    │
├──────────────┬──────────────┬──────────────┬────────────────┤
│ Market       │ Wallet       │ Orders       │ Reconciliation  │
│ discovery    │ & signing    │ & execution  │ & recovery      │
└──────┬───────┴──────┬───────┴──────┬───────┴───────┬────────┘
       │              │              │               │
┌──────▼──────────────▼──────────────▼───────────────▼────────┐
│ XRPL adapter                                                │
│ WebSocket subscriptions · JSON-RPC requests · tx submit     │
└──────────────────────────────┬──────────────────────────────┘
                               │
                    public XRPL endpoint(s)

       ┌───────────────────────────────────────────────────────┐
       │ Local persistence: SQLite + OS credential/key store   │
       │ market cache · candles · wallet metadata · tx journal  │
       └───────────────────────────────────────────────────────┘
```

## Responsibilities

The current implementation has completed the initial domain/configuration layer, a JSON-RPC `book_offers` client, WebSocket book and validated-ledger subscription clients, an XRP/token pair-discovery component, reconnecting book/ledger ingestion services, a durable JSONL event journal, versioned Go read models, and a fixture-backed React/TypeScript market screen. The application service and live UI transport are still pending.

### Frontend

The frontend is presentation and interaction only. The current screen uses fixture data; the intended transport is a local typed API/event stream. It never holds a seed, constructs an unsigned transaction from untrusted display text, or treats an RPC acknowledgment as finality.

### Application service

Coordinates use cases such as `DiscoverPairs`, `GetMarketSnapshot`, `UnlockWallet`, `CreateOffer`, `CancelOffer`, and `SendPayment`. It enforces network boundaries, validation, confirmation requirements, authorization state, and event ordering.

### XRPL adapter

The adapter owns protocol details: endpoint connections, WebSocket subscriptions, JSON-RPC calls, ledger markers, amount encoding, transaction autofill, local signing, submit-only submission, and validated-result lookup. The read-only `ReadClient` boundary is in place, with a live implementation using JSON-RPC `book_offers` and WebSocket `subscribe`. It is replaceable so tests can use fixtures or a local node.

### Market subsystem

Consumes validated ledger and book events, discovers XRP/token pairs, maintains snapshots, derives trades/spreads/depth, and aggregates candles. Each asset is represented by a canonical identity; ticker symbols are display metadata only.

### Wallet subsystem

Separates public wallet metadata from encrypted secret material. Supports watch-only wallets, encrypted imported/generated wallets, unlock sessions, balances, trust lines, reserve calculations, and local signing. Secret access is narrow, audited, and never included in application events or logs.

### Order subsystem

Owns quote validation, OfferCreate/OfferCancel construction, fee and price bounds, signed-blob journaling, submission, and lifecycle reconciliation. Order state is a state machine driven by validated ledger evidence.

### Persistence

The current persistence primitive is an append-only `storage.FileJournal` using local JSONL, with sequence cursors, flush-on-append, and replay. SQLite remains the planned embedded aggregate store. Suggested tables include `networks`, `assets`, `pairs`, `market_events`, `candles`, `wallets`, `encrypted_secrets`, `orders`, `transactions`, and `schema_migrations`. Use migrations and WAL mode when SQLite is introduced; keep raw protocol payloads only where useful for replay/debugging and redact sensitive data.

## Realtime data flow

1. Connect to the selected endpoint and record connection health.
2. Load a book snapshot and current ledger marker.
3. Subscribe to relevant book/account/ledger streams.
4. Normalize each event and append it to the local event journal.
5. Update in-memory read models and persist durable aggregates.
6. Publish typed updates to the UI.
7. On disconnect, reconnect with bounded exponential backoff, resubscribe, detect the ledger gap, backfill, then mark the stream current.

The pair watcher uses the same ingestion pipeline, filtering for newly observed XRP/token books and emitting a `PairDiscovered` event. It must be throttled, deduplicated, and observable; it must never place orders.

The current `PairDiscovery` component accepts XRPL book-change asset labels such as `XRP_drops` and `rIssuer/USD`, canonicalizes the pair to XRP as base, and deduplicates reverse observations. `market.Ingestor` owns a book stream, emits an initial normalized snapshot, forwards updates, and reloads a snapshot after disconnect before publishing recovered data. `market.LedgerIngestor` provides the validated-ledger cursor with the same reconnect behavior. Both can persist normalized events through the append-only `storage.FileJournal`, which assigns durable sequence cursors and supports replay after restart. Ledger gap detection and SQLite-backed aggregate storage remain future work.

## Transaction flow

```text
user review → validate network/assets/limits → autofill cautiously
→ sign locally → persist signed blob → submit blob
→ poll/subscribe for validated hash → reconcile result → update UI
```

An RPC response with `status: success` means the request was accepted by the server, not that the transaction executed successfully. Final state comes from validated transaction lookup and metadata.

## API boundary

Expose a versioned local API with request/response types generated or shared between Go and TypeScript. Use commands for mutations and events for asynchronous state changes. Minimum event categories:

- `ConnectionChanged`
- `LedgerAdvanced`
- `PairDiscovered`
- `BookUpdated`
- `CandleUpdated`
- `BalanceUpdated`
- `OrderUpdated`
- `TransactionUpdated`

Keep the API transport replaceable: desktop bindings initially, local HTTP/WebSocket in browser development mode, and a future CLI/client without changing domain services.

The initial Go read models live under `api/v1`. They represent financial quantities as decimal strings, keep issuer-qualified assets in every market-facing model, and use explicit order and transaction lifecycle states. The Vite web workspace currently mirrors the read-only market models in `web/src/api.ts`; it will switch to the local API transport when that boundary is exposed.

## Local development

From the repository root:

```bash
go test ./...
go run ./cmd/trader
go build -o bin/trader ./cmd/trader
```

For the fixture-backed web screen:

```bash
cd web
npm install
npm run dev
```

The web workspace also supports `npm run build`, `npm run lint`, and `npm run format`.

The default configuration targets XRPL testnet and read-only mode. The live client is currently a package-level integration boundary; it is not exposed as a trading command and does not handle wallet secrets.

## Security model

- Default to testnet and read-only mode.
- Keep seeds/private keys in the OS credential store or an encrypted database envelope; require an explicit unlock session.
- Sign on the local machine; submit only serialized signed blobs.
- Validate issuer, currency, destination, amount, fee ceiling, expiration, and slippage/price bounds immediately before signing.
- Mask sensitive values and exclude them from logs, telemetry, crash reports, and UI events.
- Make backup/export an explicit, warned, encrypted operation.

## Future extension points

- `Asset` supports native XRP, issued currencies, and future MPT/token forms.
- `Pair` is asset-agnostic even though the first filter is XRP/token.
- `MarketDataProvider` can add AMM data or other XRPL sources.
- `ExecutionVenue` can evolve from CLOB offers to path payments/AMMs while preserving order review and reconciliation.
- `Signer` can later support hardware or external signing without weakening the local default.
- Strategy and automation APIs can be added behind permissions and explicit user activation.
