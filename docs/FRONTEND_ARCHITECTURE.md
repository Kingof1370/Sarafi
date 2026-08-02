# VELYXORA EXCHANGE - FRONTEND ARCHITECTURE
Our client is built on Next.js 14 and structured React components:
- **State Store:** Uses Zustand (`src/store/authStore.ts`) to manage authentication states.
- **API Client:** Axios-based instance (`src/lib/api.ts`) managing JWT authorization injections dynamically.
- **Performance:** Prerendered static content optimizes First Load JS times (~109KB total bundle size).
