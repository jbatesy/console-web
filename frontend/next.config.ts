import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Emit a fully static site to `out/`, which the Go server embeds via go:embed
  // and serves exactly as it served the old vanilla frontend.
  output: "export",
  // Produce directory-style routes (out/jobs/index.html) so a plain file server
  // can resolve /jobs/ without per-request rewriting.
  trailingSlash: true,
  // Don't 308-redirect non-slash paths. This only affects the dev server's
  // runtime behavior (the static export has no server); without it, dev rewrites
  // for /api and /ws get redirected to a trailing slash the Go mux rejects, which
  // a WebSocket upgrade cannot follow.
  skipTrailingSlashRedirect: true,
  // next/image optimization requires a server; disable it for static export.
  images: { unoptimized: true },
};

// Dev-only proxy: keep the browser same-origin (:3000) by forwarding API/WS to
// the Go backend, so cross-origin CORS never applies. Rewrites are not emitted
// into the static export, so this is gated to `next dev` to keep prod clean.
if (process.env.NODE_ENV === "development") {
  const backend = process.env.NEXT_PUBLIC_BACKEND_ORIGIN ?? "http://127.0.0.1:8080";
  nextConfig.rewrites = async () => [
    { source: "/api/:path*", destination: `${backend}/api/:path*` },
    { source: "/ws/:path*", destination: `${backend}/ws/:path*` },
  ];
}

export default nextConfig;
