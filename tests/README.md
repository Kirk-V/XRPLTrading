# Test fixtures and integration tests

This directory is reserved for XRPL testnet integration tests, recorded/replayable ledger events, and end-to-end transaction fixtures. Unit tests currently live beside their Go packages; the XRPL adapter tests use local mock HTTP servers with realistic wire payloads. The web screen now consumes the local Go service rather than fixture data.

When adding fixtures:

- Prefer sanitized, deterministic payloads with documented network and ledger ranges.
- Keep replay data small and useful for reconnect, gap, and normalization tests.
- Never store real wallet secrets, private keys, mainnet credentials, or unredacted personal data.
