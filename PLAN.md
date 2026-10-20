# XRPL Trading Tool — Plan of Attack

## Objective

Build a portable, local-first XRPL trading workstation that avoids third-party trading-site fees while preserving user control over keys, execution, and data. Start with XRP ↔ issued-token markets; model assets and markets so token ↔ token can be added without redesigning the core.

## Technology decision

Use Go for the core service and React + TypeScript for the UI. Package the application as a desktop executable using a small Go/web shell, currently Wails is the leading candidate. Keep the Go service independently runnable so it can also power a browser-based development UI and future CLI/automation clients.

Why this is the best fit:

- A local Go process is easy to ship as one binary, has strong concurrency and networking support, and matches the desired portability.
- A web UI gives us responsive layouts, charting, accessible components, and a large ecosystem.
- Local signing and OS-backed secret storage are easier to enforce in a desktop application than in a hosted web app.
- Python remains welcome for data-analysis notebooks, replay tools, and research prototypes, but it should not be the runtime dependency for the shipped product.

Do not begin with a cloud backend. It would add custody, authentication, uptime, and operational costs to a tool whose central value is local control.

## Scope

### Initial release

1. Select XRPL network and endpoint; support testnet first.
2. Discover XRP/token pairs and identify valid asset identities by currency plus issuer.
3. Maintain live order-book and ledger-derived market data with reconnect and backfill behavior.
4. Render responsive price charts, depth, spread, volume/activity, and connection status.
5. Add/remove wallet profiles without exposing secrets; show balances, reserves, trust lines, and spendable funds.
6. Add funds through a clear receive-address workflow and remove funds through reviewed payments.
7. Execute immediate trades with explicit amount, price-impact/slippage, fee, and confirmation review.
8. Create, list, and cancel limit offers; track submitted, validated, partially filled, filled, expired, and failed states.
9. Continuously watch for newly observed XRP/token markets and notify the user without automatically trading.

### Explicitly deferred

- Token ↔ token pair discovery and routing.
- Cross-network bridges, lending, leverage, derivatives, copy trading, and automated strategies.
- Hosted custody, remote signing, account recovery, and third-party analytics requirements.
- Automatic trading on newly discovered pairs.

## Phased execution

### Phase 0 — foundation and safety (in progress)

- [x] Initialize Go module and baseline package layout.
- [x] Define initial domain types for `Asset` and `Pair`.
- [x] Add network configuration with testnet and read-only mode as the safe default.
- [x] Add a narrow, replaceable read-only XRPL client boundary.
- [x] Add frontend workspace, linting, and formatting.
- [ ] Add CI and cross-platform build checks.
- [x] Expand versioned API schemas with `Quote`, `Candle`, `Wallet`, `Balance`, `Order`, and `Transaction`.
- Decide on encrypted local storage and OS keychain/credential-manager integration.
- Write threat model and transaction review UX before enabling signing.

Exit condition: the app starts, has no secret material in logs, and can be reset safely.

### Phase 1 — read-only market vertical slice (in progress)

- [x] Implement XRPL WebSocket/JSON-RPC adapter behind an interface.
- [x] Fetch order books through `book_offers`; normalize XRP and issued-currency identities.
- [x] Implement XRP/token pair discovery and direction deduplication.
- [x] Add a market ingestion service that owns subscriptions, reconnects, and snapshot backfill.
- [x] Add validated-ledger ingestion with reconnect handling.
- [x] Add a durable event/replay model so reconnects do not silently create chart gaps.
- Render pair search, market detail, order book, trades/activity, and basic time-series charts.

Exit condition: a testnet user can select an XRP/token pair and see live updates after endpoint interruption and recovery.

### Local development commands

```bash
go test ./...
go run ./cmd/trader
go build -o bin/trader ./cmd/trader
```

The adapter uses XRPL API v2-shaped requests, `book_offers` for snapshots, and WebSocket `subscribe` requests with `books` for updates. The executable remains read-only until wallet and transaction safety work is complete.

### Phase 2 — wallets and balances

- Add wallet creation/import/watch-only profiles.
- Store only encrypted secrets, with explicit unlock/lock behavior and automatic timeout.
- Display XRP balance, reserve, spendable amount, issued-token balances, and trust-line state.
- Add receive workflow and payment sending with destination validation and final review.

Exit condition: wallet operations work end-to-end on testnet and all sensitive fields are masked by default.

### Phase 3 — order execution

- Build and validate OfferCreate and OfferCancel transactions.
- Sign locally, persist the signed transaction before submission, submit as a blob, and monitor validated outcome.
- Handle sequence management, fee autofill/limits, retries, duplicate submission, partial fills, and failures.
- Add dry-run/simulation mode and a prominent testnet/mainnet boundary.

Exit condition: limit offers can be safely created/canceled and their final ledger result is unambiguous.

### Phase 4 — release hardening

- Add migration-tested SQLite persistence, structured logs, diagnostics export, and crash recovery.
- Test Linux, Windows, and macOS packaging, upgrades, offline startup, endpoint rotation, and clock differences.
- Add fuzz/property tests for amount parsing, issuer/currency identity, order validation, and transaction state machines.
- Conduct a security review before mainnet enablement.

Exit condition: reproducible release artifacts and a documented recovery/runbook exist.

## Pair discovery strategy

Treat a pair as `(base_asset, quote_asset)` where an issued asset is `(currency_code, issuer)`. For now, only accept one side as native XRP and the other as an issued token. Observe new books and relevant validated transactions, normalize direction, deduplicate identities, and retain first-seen/last-seen metadata. Discovery is informational; it must not imply liquidity quality or safety.

Later, token ↔ token support should only require widening the pair filter and adding route selection—not changing storage, charting, or order lifecycle models.

## Design and engineering standards

- Local-first and least privilege: no secret leaves the machine; read-only mode should work without wallet unlock.
- Testnet-first: mainnet is opt-in and visually unmistakable.
- Explicit confirmation: show exact asset issuers, amounts, fee, destination, price limits, and network before signing.
- Decimal-safe finance: use fixed-precision/decimal types, never binary floating point for money.
- Idempotent workflows: every submitted transaction has a durable local record and can be reconciled by hash.
- XRPL truth wins: UI state is derived from validated ledger outcomes, not merely successful RPC responses.
- Resilient realtime: reconnect with backoff, detect gaps, resubscribe, and backfill before marking data current.
- Accessible and responsive: keyboard support, readable contrast, reduced-motion support, and usable layouts from laptop to small window.
- Observable but private: structured logs and metrics exclude seeds, secrets, and unnecessary personal data.
- Small interfaces: isolate XRPL protocol code behind ports so alternative endpoints, replay fixtures, and future protocols are testable.
- Secure defaults: no automatic trading, no plaintext exports, bounded fees, and conservative transaction timeouts.
- Reproducible delivery: pinned dependencies, automated tests, signed release artifacts where practical, and migration versioning.

## Main risks and mitigations

| Risk | Mitigation |
|---|---|
| Stolen seed/private key | OS-backed key protection, encrypted storage, short unlock lifetime, no remote signing |
| Misidentified token/issuer | Display and validate full issuer identity; never identify assets by ticker alone |
| Stale or misleading price | Show ledger timestamp/age, connection state, liquidity, spread, and data source |
| Transaction accepted but not executed | Persist signed blob and reconcile with validated transaction result |
| XRP reserve prevents operation | Calculate reserve and spendable funds before signing; explain trust-line implications |
| Endpoint outage or rate limiting | Multiple configurable endpoints, backoff, gap detection, and replayable fixtures |
| Newly found scam/illiquid token | Discovery is not endorsement; add risk indicators and no auto-trade behavior |

## First implementation tickets

1. [x] Create Go and web workspace; CI matrix remains pending.
2. [x] Add network config and a read-only XRPL client interface.
3. [x] Implement asset/pair normalization with table-driven tests.
4. [ ] Implement order-book snapshot plus streaming update reconciliation.
5. [x] Build the first fixture-backed pair list and market-detail screen.
6. [ ] Add a testnet integration harness and recorded event fixtures.
7. [ ] Decide and document storage/keychain libraries after a small cross-platform spike.

### Next-session handoff

The UI now connects to a local read-only Go HTTP service. Run `npm --prefix web ci`, `npm --prefix web run build`, and `go run ./cmd/trader`, then open http://localhost:8080. It supports discovery from sampled validated ledgers on the selected network, search/manual currency-and-issuer selection, two-sided validated book snapshots, real connection/error/empty states, and session midpoint charts. Both book directions use the same ledger; funded quantities and prices use decimal-safe normalization. Adapter tests cover actual JSON-RPC envelopes, errors, empty books, and normalization.

This first integration polls full snapshots about every five seconds; it does not consume the WebSocket ingestors or persist charts to the journal. Trading-activity discovery is sampled; AMM creation discovery uses the continuous stream described below. Both are session-only. Next steps are real WebSocket transaction reconciliation, ledger gap detection/backfill, durable aggregate storage, and a recorded integration harness. Do not enable wallet secrets or signing yet. CI and desktop packaging remain pending.

AMM discovery now listens continuously for successful validated XRP/token `AMMCreate` transactions, independently of sampled trading activity. The UI shows new pools and stream health, including interruption warnings. Reconnect/filtering/cancellation tests use mock WebSocket servers. Historical discovery backfill and AMM reserve/pricing views remain pending. Manual watches are still session-only and refresh only the selected market; a saved watchlist with background collection is a separate next step.

Read-only mainnet viewing is now available with `go run ./cmd/trader -network mainnet`. Testnet remains the default; no signing or execution has been enabled. HTTP polling and AMM WebSocket discovery use matching endpoints for the selected network, and the UI labels the network explicitly.

The market list now has All, New AMMs, and Active AMMs filters with pair/issuer/currency search. Active status is checked from validated `amm_info` for known pairs and manually entered markets, with progress shown and periodic refresh. Compact readable labels keep raw identities available. Discovering all historical AMMs and saving a watchlist remain follow-up work.
