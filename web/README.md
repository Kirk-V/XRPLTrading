# Web UI

React + TypeScript + Vite, connected to the local Go service. Use Node.js 22.13+ or Node 24 LTS.

## Run

From the repository root:

```bash
npm --prefix web ci
npm --prefix web run build
go run ./cmd/trader
```

Open http://localhost:8080. For hot reload, keep Go running on port 8080 and run `npm --prefix web run dev` in another terminal; open the printed Vite URL.

## Checks

```bash
npm --prefix web run build
npm --prefix web run lint
npm --prefix web run format
```

## Data boundary

`src/api.ts` mirrors the market read models under `api/v1`. The browser polls `GET /api/v1/state` and `GET /api/v1/book?currency=...&issuer=...` through the same origin every five seconds after each completed request. Requests are canceled on unmount/market switches and have timeouts. Vite proxies `/api` to Go during development.

The screen shows sampled market discovery on the selected network, manual market selection, both sides of a validated book, funded sizes, best bid/ask/spread, and up to 120 session midpoint samples. Prices stay decimal strings except when calculating chart coordinates. Calculated display values are rounded to 16 significant digits.

All prices come from the local service; no sample market, trade activity, or fake connection indicators remain. Quiet markets show empty states. Failures retain and label the last successful book as stale. Charts are observations, not historical candles, and do not backfill outages. Wallets/signing are not connected.

The sidebar also shows a separate AMM listener status and the newest 10 validated XRP/token pool creations received by the Go WebSocket listener. Interrupted discovery is flagged; there is no historical backfill. Selecting a new pool opens its limit order book, not AMM reserves or a swap quote. Manual watches are session-only, and only the selected market refreshes.

For mainnet, run the Go service with `go run ./cmd/trader -network mainnet`. The frontend takes its network label from the API and clears manual watches, selection, and chart state when it detects a network change. Mainnet remains read-only; the default without a flag remains testnet.

See [How market discovery works](../README.md#how-market-discovery-works) for discovery sources, retention limits, missed-event behavior, and manual-watch semantics.
