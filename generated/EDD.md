# Garage CRM — Code-derived engineering evidence

**Evidence status:** Development build, not release acceptance  
**Authoritative source-changing GitHub commit:** `99da8cc802de11beea90e955c07b0422cbdf2220`  
**Target design:** locked `PRD.md` / `EDD.md` version 0.4.2  
**Build identifier:** `0.1.0-dev`

This document describes what the cited source actually implements. It is not a substitute for the normative v0.4.2 EDD and does not assert that all its acceptance cases have been completed.

## Runtime

- `cmd/garage-crm/main.go`: Go CLI with serve, backup, export, inspect and clean-target import. Default data root `$XDG_DATA_HOME/garage-crm` or `~/.local/share/garage-crm`. OS file lock prevents simultaneous processes on a single data directory. HTTP TCP4 bound to `127.0.0.1` only. Desktop opening via optional system `xdg-open`.
- `internal/sqlite/sqlite.go`: direct system libsqlite3 through cgo, SQLite prepared SQL, transactions, explicit locking, foreign keys, WAL, `synchronous=FULL`, secure deletion setting, online SQLite backup. No ORM, server/database pool, broker, external API or Node runtime.
- `internal/app/schema.go`: 34 schema tables (33 business tables in the portable archive, with migration metadata represented by the manifest) with FK links, revision/audit history, brand-specific relationship data, opportunity stage events and stored attachment BLOBs. Single database is the authoritative source.
- `internal/app/model.go` and `internal/app/workflows.go`: typed entity CRUD, brand validations, optimistic revisions, lead conversion, stage changes, activities with follow-ups, opportunity lines, campaign recipient snapshot, manual outcomes and recipient opt-out re-checking, basic reporting and task recurrence.
- `internal/app/relationships.go`: brand-specific contacts, contact merge with restriction-preserving permissions, aliases, multiple contact channels and organisation affiliations, offer relationships and permanent-contact PII redaction.
- `internal/portable/archive.go`: versioned `garage-crm-export/1` ZIP with manifest, all stored entities, checksums and bytes of SQLite attachment BLOBs, and import to a new empty database with FK checks and safe failure. Not an in-place overwrite/restore engine.
- `internal/web/web.go`, `internal/web/people.go`, `internal/web/template.go` and `internal/web/static`: server-rendered forms with embedded CSS/SVG, localhost Host check, per-run bootstrap token in owner-only local starter file, HttpOnly SameSite cookie, Origin/CSRF validation, escaped HTML, download attachment headers and bounded uploads. There is no JavaScript build or CDN.

## Security trust boundary

- Browser traffic on loopback; OS owner-only data directory, SQLite and private starter file; short-lived one-use bootstrap token, CSRF defence, restrictive CSP, no remote fonts/assets. Session is local access protection, not IAM.
- SQL values bound; table and column names fixed to allow-listed schema code; `html/template` escapes CRM content; untrusted attachments downloaded as `application/octet-stream` with no sniffing and attachment disposition.
- Campaign recipient lists check current brand consent, brand opt-out and global DNC at download time; historical snapshots persist. Personal deletion redacts tested data across linked histories and excludes it from subsequent archives. Prior backups and SSD blocks cannot be erased by this tool.
- At-rest archive/database encryption is **not** provided. Same-user malware and compromised OS remain outside the product trust boundary.

## Verified tests and limitations

Automated test source: `internal/app/app_test.go`, `internal/web/web_test.go`. Tests cover representative multi-brand and campaign opt-out, lead conversion idempotence, concurrent revision rejection, stage history, full archive with binary attachment and corrupted-input rejection, restrictive merge semantics, personal redaction roundtrip, unauthenticated GET and wrong token, bootstrap replay, cross-origin/CSRF, XSS escaping, and browser asset security headers.

Executed locally: `go mod tidy`, gofmt, `go vet ./...`, `go test -race ./...`, Linux AMD64 native build and CLI startup. GitHub CI run `43852206135` passed native AMD64 and ARM64 jobs for source commit above. The local binary dynamically links system `libsqlite3.so.0` and glibc. Ubuntu UI/visual acceptance was not demonstrated because the test browser environment blocks localhost navigation.

**Release-blocking unknowns:** full PRD feature coverage including custom fields UI, richer saved views/filters, pipeline configurability, advanced campaign attribution and workflow UX, CSV imports, in-place generation-safe restore, migration stress/performance budgets, complete accessibility, comprehensive real-client security negative tests, dependency vulnerability scan and signed stable desktop distribution. See `docs/BUILD-STATUS.md` and `generated/COMPARISON.md`.
