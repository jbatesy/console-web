// In production the static export is served by the Go server itself, so the
// backend is same-origin and this is empty. In `next dev` (separate port) set
// NEXT_PUBLIC_BACKEND_ORIGIN=http://127.0.0.1:8080 to reach the Go server.
const BACKEND_ORIGIN = process.env.NEXT_PUBLIC_BACKEND_ORIGIN ?? "";

/** Absolute or same-origin URL for an HTTP API path (e.g. "/api/jobs"). */
export function apiUrl(path: string): string {
  return `${BACKEND_ORIGIN}${path}`;
}

/** ws:// or wss:// URL for a WebSocket path (e.g. "/ws/pane/abc"). */
export function wsUrl(path: string): string {
  const origin = BACKEND_ORIGIN || window.location.origin;
  const u = new URL(origin);
  const proto = u.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${u.host}${path}`;
}
