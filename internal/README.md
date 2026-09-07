# Internal Go packages

The current packages are organized by responsibility:

- `domain`: canonical asset and XRP/token pair identities.
- `config`: testnet-first endpoint and read-only safety configuration.
- `xrpl`: replaceable read-only JSON-RPC/WebSocket protocol boundary.
- `market`: pair discovery plus reconnecting book and validated-ledger ingestion.
- `storage`: append-only local event journal with replay support.

Planned packages for the next phases include the application service, wallet, order, and SQLite persistence layers. Boundaries and security expectations are described in [ARCHITECTURE.md](../ARCHITECTURE.md).
