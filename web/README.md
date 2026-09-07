# Web UI

The React + TypeScript frontend is a Vite workspace. It currently contains a read-only, fixture-backed testnet market screen and is ready to connect to the versioned Go API.

## Development

```bash
npm install
npm run dev
npm run build
npm run lint
npm run format
```

The UI intentionally has no wallet or signing code. Network state is visibly testnet/read-only until the local service boundary is connected.

## Current boundary

`src/api.ts` mirrors the initial Go read models and provides fixture data for the screen. The next integration task is a local read-only HTTP/WebSocket transport; do not add secrets, signing, or direct XRPL connections to the browser UI.
