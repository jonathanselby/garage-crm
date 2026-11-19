# Garage CRM — Development build status

**Status:** `0.1.3-dev`, not an accepted Version 1 release. **Locked contract:** PRD/EDD v0.4.4 and the attached Garage GitHub Publication Learnings (9).

## Implemented, locally verified
- Single Go executable; compiled Linux AMD64 with SQLite system library.
- Brand, offer, person, organisation, lead, opportunity, campaign and task creation/edit/archive; linked brand contact relationships and roles; campaign variants, snapshot and manually logged outcomes; Kanban and My Day; manual activity + follow-up; relationship merges with restrictive consent; personal redaction with export tests.
- Read-only cooling detector for open deals and brand-linked prospects with no next action; dashboard tile, direct record links and brand-aware query; regression and UI tests added (CI evidence governs whether they pass).
- Prepared SQLite, FK constraints, revision checks, status/history, system SQLite backup; portable ZIP with all tables and binary attachments; fail-before-replace import into a fresh directory.
- Loopback listener with local one-time starter page, protected session, Host/Origin/CSRF, output escaping, forced attachment download, bounded input.
- Automated Go tests covering representative campaigns, brand suppression, conversion, stage concurrency, merge, permanent redaction + archive, corrupt import and local browser security.

The fixed 14-day beta cooling alert does not satisfy all configurable-alert or broader v1 acceptance requirements.

## Known missing or partially tested PRD requirements — release blocking
- Comprehensive CRM field UI, saved searches/views, fully typed custom fields, bulk edit/tag, duplicate suggestions/merge preview quality, complete organisation history/parent flows, contact merge conflict reconciliation and restore/undo UX.
- Fully configurable per-brand opportunity pipelines, complete stage exit checklists, product line editor quality, probabilistic forecasts and deal reports.
- Campaign segment filter builder, frozen audience amendments, balanced A/B cohorts, manually evidenced conversion attribution and detailed campaign reports.
- Task prioritisation and recurrence with full idempotency/holiday/DST test, date-grid view, persistent manual Kanban ordering and comprehensive keyboard UX.
- General CSV import with mapping/dry-run and filtered CSV export; same-store staged atomic restore/replacement, robust streaming import/export for multi-GB archives, backup/restore UX and golden compatibility fixtures.
- All permanent redaction classes validated, event/history detail coverage, comprehensive negative tests and OS permission edge cases; strict bootstrap time expiry and launch across mainstream browsers require real UAT.
- True independent desktop Ubuntu visual UAT inaccessible in current restricted browser environment; memory/CPU/latency stress budgets not measured. AppImage/ARM64 acceptance not completed.
- Go vulnerability scan/supply-chain assessment not run (offline build environment); only project tests and vet are available.

## Gates run locally (record outputs rather than imply broad coverage)

See test results in this repository and GitHub Actions checks. Scripted unit and integration checks are not equivalent to all 75+ PRD acceptance scenarios. No stable tag or release until the full gate is met.

## Distribution note

The preview dynamically uses `libsqlite3.so.0` and the system's C runtime. The executable is not yet a fully self-contained AppImage. Client data is unencrypted at rest; users should use full-disk encryption and protect portable archives.

## Development correction 19 November 2026 (design v0.4.4)

The old form handler rejected even a valid browser `Origin: null`. The source now allows opaque/missing Origin only behind the authenticated per-run browser session and correct CSRF token, and rejects explicit contradictory cross-origin provenance. Added Quit sidebar action using protected POST and graceful HTTP shutdown. `serve()` now waits for shutdown completion before database closure, addressing an earlier select-default lifetime race.

The real-listener regression test exercises bootstrap, session, form save, missing-token refusal, Quit completion and a SQLite reopen, using disposable test records. The previous v1 release blockers above are unchanged. **Do not report native Ubuntu browser or installed-package UAT complete until the user validates it.** The package is a conventional system-installed `.deb`; the Downloads directory is not the install location.
