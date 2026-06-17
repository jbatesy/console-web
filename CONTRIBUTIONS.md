# Contributing to console-web

Thanks for your interest in console-web. This document covers how to set up the
project, the conventions the codebase follows, and what's expected before a
change can be merged.

console-web is a small, personal-scale tool. Changes should keep it that way:
single binary, no required external services, minimal configuration.

---

## Prerequisites

- **Go 1.26.4 or newer** — the version is pinned in `go.mod`. CI uses
  `go-version-file: go.mod`, so match it locally to avoid surprises.
- **A Unix-like OS** — macOS or Linux. The PTY dependency
  (`github.com/creack/pty`) is Unix-only; Windows is intentionally unsupported.
- No C toolchain is needed. SQLite is provided by the pure-Go
  `modernc.org/sqlite`, so the project builds with `CGO_ENABLED=0`.
- No Node or frontend build step. The frontend is vanilla JS embedded via
  `go:embed`; xterm.js is vendored under `frontend/xterm/`.

---

## Getting started

```sh
git clone https://github.com/jbatesy/console-web.git
cd console-web
go run .
```

The server listens on `http://127.0.0.1:8080` by default. See the
[README](README.md#configuration) for runtime flags. Running the server creates
`console-web.db` and a `data/` directory in the working directory — both are
local state and should not be committed.

---

## Project layout

```
main.go                 server startup, flag parsing, static-file fallthrough
internal/
  db/        store.go    SQLite schema + persistence (jobs, sessions, panes)
  pty/       manager.go  PTY lifecycle, client broadcast, scrollback files
  session/   manager.go  session/pane creation, reconnect + alive reconciliation
  validate/  vars.go     per-variable regex validation, {{var}} substitution
  api/       handlers.go HTTP handlers (jobs REST, launch, sessions)
             ws.go       WebSocket ↔ PTY bridge
frontend/                embedded UI (index/editor HTML+JS, style.css, xterm/)
docs/                    design specs and implementation plans
```

Keep packages focused: persistence stays in `db`, process management in `pty`,
HTTP concerns in `api`. Note that `session` re-implements `{{var}}` substitution
locally to avoid a circular import with `validate` — keep the two in sync if you
change substitution semantics.

---

## Development workflow

1. Branch off `main`. Don't commit directly to `main`.
2. Make your change with a matching test (see below).
3. Run the full local check suite before pushing:

   ```sh
   gofmt -l .          # must print nothing
   go vet ./...
   go test -race ./...
   go build ./...
   ```

4. Open a pull request. CI runs the same checks; all must pass.

### Commit messages

Follow the existing history's
[Conventional Commits](https://www.conventionalcommits.org/) style:

```
fix: clear Content-Type header before file server fallthrough
chore: untrack superpowers working dirs; keep them local
```

Use a type prefix (`feat`, `fix`, `chore`, `docs`, `refactor`, `test`), an
imperative summary, and a body when the "why" isn't obvious.

---

## Testing

Tests live alongside the code they cover (`*_test.go`) across `internal/api`,
`internal/db`, `internal/pty`, `internal/session`, and `internal/validate`.

- **Add or update tests for every behavioral change.** New endpoints, validation
  rules, or PTY behavior need coverage.
- **Run with the race detector.** The PTY/WebSocket/session code is concurrent;
  `go test -race ./...` is the bar that CI enforces and that catches the bugs
  that matter here.
- Prefer table-driven tests, matching the style already in the package.

```sh
go test ./...                          # all packages
go test -race ./internal/pty           # one package
go test -run TestTrimScrollback ./...  # one test
```

---

## Code style

- **Formatting is non-negotiable:** `gofmt` must be clean. CI fails on any file
  listed by `gofmt -l .`.
- Pass `go vet ./...` with no findings.
- Match the surrounding code's naming and structure. Exported identifiers get doc
  comments (`// Name ...`); look at existing files for the cadence.
- Handle and wrap errors with context (`fmt.Errorf("create session: %w", err)`),
  as the existing code does.

### Security-sensitive areas

This tool runs arbitrary shell commands and has no authentication, so be
deliberate around:

- **Variable handling.** Raw variable values must never reach the shell directly.
  Only the fully substituted command string is passed to `bash -c`, and only
  after server-side regex validation. Don't send substituted commands to the
  browser.
- **The HTTP/static-file fallthrough** in `main.go` deliberately scrubs headers
  and intercepts 404s; understand it before changing routing.
- **WebSocket framing and broadcast** in `internal/pty` and `internal/api/ws.go`
  are concurrent and shared across clients — test changes with `-race`.

---

## Documentation

- Update the [README](README.md) when you change flags, the API surface, or
  user-facing behavior.
- Design specs and plans live under `docs/`. For larger changes, a short design
  note there is appreciated.

---

## Roadmap

console-web started as a minimal single-user tool, but we want to grow it.
Contributions toward the [README's TODO list](README.md#todo) are especially
welcome:

- **Authentication / access control** — so it can run somewhere other than a
  trusted localhost.
- **Remote execution** — running commands on machines other than the server
  host.
- **Windows support** — currently blocked by the Unix-only PTY dependency; would
  need a cross-platform PTY abstraction.

For larger features like these, open an issue first so we can agree on the
approach before you invest in a PR. Keep the core ethos in mind — easy to run,
few required dependencies — but don't treat the current limitations as
permanent: they're things we'd like to fix.
