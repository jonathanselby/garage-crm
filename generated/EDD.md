# Garage CRM — Code-derived engineering evidence

**Evidence status:** Tested development build, not full Version 1 acceptance
**Authoritative final source-changing commit:** `b5c1b3cb00a8d6d64704ff272c0421f8da5edf46`
**Source tree SHA:** `c084fbc41a52b9f16ee477d7aff89f851a75db79`
**Design baseline:** locked `PRD.md` / `EDD.md` version **0.4.4**
**Development build:** `0.1.3-dev`
**CI verification:** [run 44451947256](https://github.com/jonathanselby/garage-crm/actions/runs/44451947256) — Linux AMD64 and ARM64 completed successfully for the source-changing commit above.

## Deployed execution shape (source inspection)

- `cmd/garage-crm/main.go`: one local Go process; serve, backup, export, import-to-empty, inspect and version commands. It acquires an exclusive data-directory process lock, binds `127.0.0.1`, launches a private starter page where possible, and owns the HTTP listener and SQLite database lifetimes.
- `internal/web/web.go`, `internal/web/template.go`, `internal/web/static/`: Go server-rendered HTML and CSS/SVG, no SPA/runtime frontend dependency. Per-run one-use private bootstrap issues an HttpOnly SameSite session cookie; all normal POST operations require a valid session, a per-run CSRF secret and provenance validation. Explicit hostile Origin, contradictory Referer and cross-site Fetch Metadata are refused. The browser can omit Origin or send `null`; this case is gated by session and CSRF rather than Origin alone.
- Quit is a browser-visible form in the sidebar posting `/quit` with the current CSRF token. The handler returns a closed page and signals the owner. `http.Server.Shutdown` is bounded and `serve` waits for shutdown completion before its deferred SQLite close and process-lock release.
- `internal/app/`: domain services for contacts, organisations, brands, products, leads, opportunities, campaigns, tasks, permissions, cooling and history. Business rules and transactions are outside HTML presentation.
- `internal/sqlite/sqlite.go`: one authoritative system SQLite store, cgo library, prepared statements, transactions, WAL and owner-only data directory; no external database or network API.
- `internal/portable/archive.go`: portable ZIP export/inspect and import to a new empty data directory; not an in-place atomic restore engine.

## Distribution and version ownership

The one authoritative application build string is `VERSION` = `0.1.3-dev`. GitHub CI injects it through `-ldflags -X main.version` in both native builds. The web template receives the same running-binary value through the view, and the package derives Debian version `0.1.3~dev` from the file. The application installs to `/usr/bin/garage-crm`; desktop entry at `/usr/share/applications/garage-crm.desktop`; icon under `/usr/share/icons/`. Writable data remains under `$XDG_DATA_HOME/garage-crm`, otherwise `~/.local/share/garage-crm`. No customer database is shipped in the package.

## Tests actually executed

GitHub Actions CI for commit `b5c1b3cb00a8d6d64704ff272c0421f8da5edf46` completed the following:
- AMD64: `go mod tidy` with module-file comparison; Go formatting check; `go vet ./...`; `go test -race -count=1 ./...`; native Linux AMD64 binary build and version check; Ubuntu `.deb` construction and `dpkg-deb --info` check.
- ARM64: `go vet ./...`; `go test -race ./...`; native Linux ARM64 binary build and version check.
- New `cmd/garage-crm/main_test.go`: starts a real loopback HTTP listener with disposable data; reads the owner-only bootstrap file; acquires the session cookie; obtains the real CSRF token; confirms a missing token is refused; saves a record with `Origin: null`; submits protected Quit; waits for server termination; reopens SQLite and confirms the record persisted.
- `internal/web/web_test.go`: same-origin forms, opaque/missing origin, missing CSRF, foreign Origin, hostile Referer/Fetch Metadata, unauthenticated and unauthorised Quit, and replay/Host controls. Other existing app, portable archive, cooling and XSS tests remain part of the full test suite.

**Limits of evidence:** CI is not a complete desktop/browser UAT and not a proof that all PRD requirements are present. Native Ubuntu package upgrade, real end-user Chrome/Chromium save/quit flow, visual accessibility, high-volume load/latency, data restoration under failure, complete security assurance and all other `docs/BUILD-STATUS.md` release blockers remain outstanding. The application is not a security-certified or complete v1 product.

## Final-source pinning policy

This evidence is documentation-only and refers to the final source-changing commit above. The evidence commit does **not** modify the source, VERSION, tests, or CI. The historical `v0.0.1` release remains immutable. Earlier micro-commits survive only on the explicitly named archived recovery branch.
