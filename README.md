# Garage CRM

An offline-first, single-user CRM for Linux, built as one Go application with SQLite and a polished local browser interface.

**Build status:** `0.1.0-dev` — development build, not a v1 release. The governing product and engineering specifications are `PRD.md` / `EDD.md` v0.4.2. The implemented subset and the outstanding acceptance gates are documented in `docs/BUILD-STATUS.md`; the application must not be described as functionally complete or security certified.

## What currently works

- Brands/business lines, contacts, organisations, leads, opportunities, products, campaigns and tasks.
- Local record creation, editing, archiving and revision-checked updates.
- Multi-brand contact relationships and manually recorded contact permission; suppression checked again on campaign recipient CSV export.
- Lead conversion to an opportunity, opportunity stage history and optional offer line items.
- Manual campaign audience snapshot, A/B copy variants and recipient outcome logging.
- My Day dashboard, task Kanban, notes and next-task logging, basic pipeline reporting.
- Bounded attachments served as forced downloads; portable full-data archive including attachment bytes.
- Loopback browser access with per-run private bootstrap, HttpOnly session cookie, CSRF and Origin protections.

## Running locally

The pre-release Linux amd64 binary requires **libsqlite3** installed (standard on Ubuntu); the source build needs Go 1.23+ and `libsqlite3-dev`.

```text
./garage-crm serve
```

The application opens the local browser automatically when `xdg-open` is available. If not, open the printed private starter HTML file in your own browser. Do not share that file. Writable data is under `~/.local/share/garage-crm/`, separate from the executable.

Other commands:

```text
garage-crm version
garage-crm export -output /private/path/garage-crm.zip
garage-crm inspect -input /private/path/garage-crm.zip
garage-crm import -input /private/path/garage-crm.zip -data /new/private/directory
garage-crm backup -output /private/path/garage-crm-backup.db
```

Do not import into a populated data directory. The current build's portable restore only accepts an empty destination and has not yet implemented the locked specification's in-place staged-generation promotion.

## Tests

```text
go mod tidy
gofmt -l ./cmd ./internal
go vet ./...
go test -race ./...
go build ./cmd/garage-crm
```

SQLite links through cgo to the system SQLite library. GitHub CI must test separately on each supported target architecture; this is not yet a self-contained AppImage. No network access is required at runtime.

## Scope and engineering doctrine

`PRD.md` and `EDD.md` are the frozen v0.4.2 target. `docs/Garage-Publication-Method.md` governs build and publication. We follow [engineering-under-constraints](https://github.com/jonathanselby/engineering-under-constraints): smallest sufficient design, visible state, no speculative infrastructure, proven recovery and real-environment tests.

V2 may introduce multi-user operation, scoped model assistance and optional external connectors after full versioned export/import compatibility is proven. None of that infrastructure belongs in v1.

## Security boundary

The application listens only on 127.0.0.1. The Linux account and its filesystem permissions are the trust boundary; local data and exported archives are unencrypted. Do not assume that local malware running as the same account is prevented from reading them. **This is not a hardened or security-certified release.**