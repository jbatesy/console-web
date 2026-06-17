// The session id lives in the URL fragment (#session=<id>) after a launch
// redirect, and is cached in sessionStorage so reloads without the fragment
// still reattach. Ported from the legacy app.js getSessionId().
export function getSessionId(): string | null {
  if (typeof window === "undefined") return null;
  const hash = window.location.hash.replace(/^#/, "");
  const params = new URLSearchParams(hash);
  const id = params.get("session");
  if (id) {
    sessionStorage.setItem("sessionId", id);
    return id;
  }
  return sessionStorage.getItem("sessionId");
}
