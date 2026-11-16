# Garage CRM — Engineering Design Document

**Design version:** 0.4.2  
**Target build:** Version 1, single-user/offline Linux  
**Status:** Final functional/security clarification — lock candidate; **not** implementation-verified  
**Date:** 16 November 2026  
**Companion PRD:** `Garage_CRM_PRD_v0.4.2.md`  
**Build publication method:** `Garage_GitHub_Publication_Learnings(8).md`

## 1. Mission, constraints and engineering doctrine

Deliver a complete, fast CRM for a single local user managing any number of brands, products, leads, contacts, opportunities, campaigns and tasks. Use the smallest sufficient design: explicit state, one authoritative store, deterministic decisions, bounded resource usage, recovery designed before failure, and tests in the actual environment. Prefer deletions of infrastructure over speculative layers. Functionality in the PRD is not negotiable merely to simplify code.

**Hard v1 boundaries:** Ubuntu desktop, one Go application process, loopback browser, SQLite and local files; no network dependencies, IAM, cloud, background daemon, connectors, model harness, tool/messaging provider, external public API, plugin runtime, frontend build chain or generic workflow language. No artificial count limits for brands/offers/campaigns.

**Future promise limited to portable data:** V2 has planned users, model harness and integrations, but v1 implements **only** sound domain boundaries, stable IDs, structured history and a versioned portable archive. Future adapters are not written now.

## 2. Architecture and component removal proof

```
Ubuntu OS user / local browser
         |
         | loopback HTTP + per-run browser token
         v
One Go executable
  cmd/garage-crm        startup/shutdown, launcher
  internal/httpui       server-rendered HTML, forms, CSRF, embedded CSS/SVG/JS
  internal/domain       lead conversion, contact merge, campaigns, offers, tasks, rules
  internal/store        prepared SQL, transactions, migrations, import/export/backup
         |
         v
SQLite WAL database, including a separate attachment BLOB table
         |
         +-- user-chosen full portable archive / recovery backup
```

- Only the first-party browser communicates with the loopback app. Templates, icons, fonts (prefer system), styles and scripts are embedded with `go:embed`; no CDN, embedded webview framework or separate server.
- Go standard library where feasible; one maintained, security-appropriate SQLite driver (pure Go only if correctness and measured footprint justify it); SQL and explicit transactions, not an ORM. Use a small reader pool with one coordinated writer, busy timeout and transaction cancellation; set `PRAGMA foreign_keys=ON` on **every connection**, use WAL, and set/verify `synchronous=FULL` for the documented power-loss durability contract. Measure any write-latency trade-off rather than silently weakening durability. No JavaScript package manager required; small vanilla JS for optional progressive enhancements.
- One writable local data root, normally under XDG data path (e.g. `~/.local/share/garage-crm/`); safe user-selectable data location supported through a validated move. Desktop launcher opens browser and never stores data inside the executable/AppImage. Directory permissions owner-only; prevent multiple process writers using an OS lock or equivalent. Keep **one active SQLite database**; use a minimal same-filesystem staged database plus a single durable active-generation selector solely for safe restore/replacement (see §5). **Attachments are stored as bounded BLOBs in a separate SQLite table, not in parallel unmanaged filesystem trees**, so a consistent SQLite backup covers all records and file bytes. **Trade-off:** the database grows with attachments; benchmark full backup duration, 25 MiB upload memory and search/CRUD queries under attachment load. Limit concurrent attachment uploads and count copying/buffering toward the RSS budget. If measurements require a separate file store later, introduce it with a tested migration rather than speculative abstraction today.
- Components **not** included and reason: user/IAM system (one user), job broker (no asynchronous processing), connectors/API abstraction (no integrations), LLM harness (no use), event streaming (SQLite transactions suffice), distributed cache (indexes suffice), heavy SPA (server-rendered first paint), general workflow engine (typed domain operations suffice), external schedulers (user-visible on-open reminders).

## 3. Authoritative data ownership and model

All primary records have **stable opaque string IDs**, created/updated timestamps and archival state as appropriate. IDs survive merge, archive, full export and import. Do not infer brand or user identity from string names. Use SQLite foreign keys ON, constraints, unique keys, migrations and indexes; assert PRD invariants at both domain and data layers where supported.

| Domain | Principal tables (logical; actual SQL may group technical metadata) | Invariants |
|---|---|---|
| Businesses | `brands`, `offers`, `pipelines`, `pipeline_stages` | offer belongs to one brand; stage belongs to a brand pipeline; never re-use archived IDs |
| Global identities | `people`, `person_emails`, `person_phones`, `organisations`, `person_organisations` | one person may have multiple organisation affiliations; original email formatting retained; suggest, do not force, deduplication |
| Brand relationships | `brand_people`, `brand_organisations`, `offer_interests` | distinct lifecycle/consent per brand, offer interest per person-brand and offer in matching brand; global DNC overrides |
| Leads | `leads`, `lead_offers`, `lead_stage_events` | each lead assigned exactly one brand; any offer linked must match; conversion preserves original lead ID/history |
| Opportunities | `opportunities`, `opportunity_lines`, `opportunity_participants`, `stage_events` | one brand/pipeline; all referenced offers same brand; all line currencies equal opportunity currency; every transition recorded |
| Marketing | `campaigns`, `campaign_offers`, `campaign_segments`, `campaign_members`, `campaign_variants`, `campaign_outcomes`, `campaign_family_labels`, `campaign_attribution` | one brand per campaign; campaign offer and recipient relationship same brand; snapshot immutable except explicit amendments; one primary attributed campaign per opportunity |
| Follow-up | `tasks`, `task_checklist`, `task_recurrence`, `activities` | nullable brand for global task; typed optional link FKs to person/company/lead/opportunity/campaign; linked brand must be consistent; recurrence key unique |
| Flexibility | `custom_field_definitions`, `custom_field_values`, `saved_views`, `settings` | fixed types; no executable expressions, code, SQL or external references; scoped definitions |
| Files and history | `attachments`, `change_events`, `schema_migrations` | original attached bytes are stored as SQLite BLOBs with sizes/digests; audit/change history is not cryptographically immutable |

**No polymorphic FK for important relationships.** Use named FK columns and/or narrow junctions for business-critical references. Where generic history rows need an entity-type label, maintain a durable tombstone/alias for merged/archived IDs and test their referential meaning. Affiliation rows hold role/start/end dates; no assumption that a person has only one employer. **Historic audience entries and outcome facts must retain their original recipient ID and event-time snapshot**, even after a merge; current recipient eligibility resolves aliases to the canonical person. A merge must not relax opt-outs or erase attribution.

**Commercial relationship examples to prove:** Person A is an interested StagZero lead, an existing consulting client, and a potential app-campaign recipient; within StagZero they can be an existing customer for Offer 1 and prospect for Offer 2. A second person at the same company may influence multiple opportunities. None requires copying a person record.

**Money:** store signed integer **minor units and ISO 4217 currency code** in every money-bearing record; each opportunity's lines must use its currency. Decimal quantity uses a canonical fixed-precision decimal representation (not binary floating point) with one documented rounding mode, checked multiplication/aggregation and an explicit error on overflow; currency minor-unit digits must be honoured. Brand default currency is a default **only**, not a data constraint. Reports with multiple currencies group values by currency; never perform silent FX conversions or combine currency amounts. Calculated total rule is `sum(lines)`, or an explicit user-authored opportunity total override with source/reason shown; `weighted_open = effective_amount × probability/100` with deterministic rounding. Campaign budget/spend also require currency. Avoid calling won opportunity amount "revenue received".

**Dates:** store UTC instants for logged activity and transitions; date-only local ISO strings for due dates/important dates and expected close where appropriate. Store the selected IANA local time zone with configuration and portable archive so "today", recurrence and DST boundaries are deterministic after restore. Preserve source time zone if imported when material. `NULL` is unknown, distinct from 0/false/empty string. Confirm selected local time zone on export/restore.

**Contact permissions:** global suppression in person; brand-specific channel opt-out/consent in brand_people. Recipient export checks both **at export time**, regardless of frozen campaign membership. On merge, preserve the most restrictive global/brand/channel suppression until explicit user review; never derive permission from another brand or silently relax an opt-out. Treat permission unknown as warning/exclusion by default; allow deliberate explicit override of **unknown status only** with visible confirmation and change history, never override an active global do-not-contact. No claim this alone meets spam/privacy law.

**Attribution:** one primary campaign ID for originating opportunity value; optional non-additive influences. Store original lead campaign/source and transition history. Campaign-family labels aggregate separately and cannot pool recipient permissions.

## 4. Business state machines and transaction boundaries

Explicit states and legal transitions, with a `from/to/when/reason` record where meaningful. **First-run invariant:** global people/organisations/tasks may exist without a brand; the first brand-scoped write atomically creates or selects a valid brand, with one inline prompt and no account wizard. Empty brand IDs are rejected on brand-scoped records:

- **Lead:** New → Contacted/Qualified/Disqualified/Nurture → Converted; requalification and manual reopening allowed only with history. Converted lead continues to exist.
- **Opportunity:** open stage(s) → Won/Lost; reopen recorded explicitly; pipeline/stage IDs validated. Stage edits do not rewrite event labels already captured.
- **Campaign:** Draft → Planned → Active → Completed/Cancelled; activation means **audience frozen for manual outreach**, never mail sent. Reactivation requires an explicit revision event.
- **Task:** Backlog/To Do/In Progress/Blocked → Done; reopening allowed. Due is derived from current date/status, not a separate mutable state. Recurrence generated idempotently on completion or next open, with no endless catch-up.
- **Archive/merge:** archived identity remains addressable by stable ID for history and export; merging keeps alias/tombstone, records conflicts, relinks live references once and is one transaction.

**Atomic workflows:**
1. Lead conversion: resolve existing global identity, validate brand/offer, create opportunity if requested, write associations and stage/history, mark converted. Use a unique conversion link/idempotency key; re-submit returns existing result.
2. Contact/organisation merge: preview conflict choices, validate brand-specific records and campaign memberships, remap **current** FK references without rewriting historical campaign recipient snapshots/recorded outcomes, retain original aliases and change record, use more restrictive suppression where there are conflicts, commit or roll back.
3. Campaign activation: resolve saved segment, cross-check brand, consent and duplicates; transactionally materialise audience/cohorts, set status and history. Idempotent repeated activation. Later manual add/remove is recorded but never alters original snapshot silently.
4. Opportunity stage or amount change: validate pipeline, stage, line currency, terminal status and amount reconciliation; save stage event and update dependent reports in one transaction.
5. Activity + task: save manual interaction and optional next task together atomically; no hidden email or call logs.
6. Recurring tasks: unique `(template_id, due_date)` occurrence key; keep **at most one next outstanding recurrence**, never auto-generate years of missed tasks, show skipped periods, and make repeated page open idempotent.
7. Attachment create/delete: validate file type/size, compute digest and store bytes in the dedicated SQLite attachment table in one DB transaction; retrieve by ID, not by untrusted path. Keep list/search queries from fetching the BLOB column. Contact search may use indexed predicates and SQLite full-text search **only if measured queries require it**; do not introduce an external search service.

All writes use transaction timeouts and present a clear user-facing success/failure result. Never silently skip an expected operation. Every editable primary record has a lightweight revision counter checked in the `UPDATE ... WHERE id=? AND revision=?` predicate; zero updated rows produce a non-destructive stale-form conflict (including two tabs), not last-write-wins. Apply this to stage moves and brand/offer edits as well as forms; no multi-user machinery is introduced. Dynamic segment filters use only a fixed allow-list of indexed fields, typed comparisons and bounded nesting; compile them to parameterised predicates, never accept raw SQL or executable expressions.

## 5. Export, import, backup, upgrades — the v2 bridge

**Portable contract:** `garage-crm-export/1`, a documented ZIP archive containing `manifest.json` with `format_version`, `schema_version`, `app_version`, exported UTC timestamp, counts, byte lengths and SHA-256 for each payload. Deterministic schema-typed UTF-8 JSONL streams, sorted by stable ID where practical, and attachment blobs by digest (exported from SQLite BLOBs). Represent NULL/empty separately. Export all domain records, aliases, custom fields, manual activity, historical pipeline transitions, campaign variant/cohort/snapshots/amendments, linked tasks, preferences and history. Exclude SQLite WAL/SHM ephemeral files and transient runtime cookie secrets.

**Full export:** obtain a consistent SQLite snapshot, stream archives rather than loading everything into RAM, write to temporary destination, verify checksums, flush and rename where supported; display path, total counts and checksum. A database backup is a different mechanism and must not be presented as v2 interoperability.

**Full import:** validate manifest, format versions, entry count, paths, compressed/uncompressed sizes, expansion ratio, digest and schema before mutation. Stage into a new private SQLite database, **including restored attachment BLOBs**, then validate all FKs, constraints, indexes and content hashes; report counts and errors. On confirmed **replace** of a populated store, quiesce the writer and close active SQLite handles, checkpoint staged and existing databases and use a **single durable active-generation selector** within the same data root: stage and fsync a complete database generation; validate it before promotion; atomically replace the selector file (fsync file and parent directory), then reopen and verify. Keep the previous verified generation for deterministic startup rollback if the selector/promotion or reopening fails. Never assume that renaming a live SQLite DB file while WAL/SHM handles exist is atomic. Implement this only for restore/replacement, not as an ongoing database-versioning framework. On any failure keep/recover the original; never partially merge a full archive in v1.

**CSV:** separate spreadsheet-safe interchange with column mapping and preview. Treat CSV fields as untrusted text; avoid spreadsheet formula injection on *export*, and do not re-import neutralised CSV as if it were the complete-fidelity format. CSV is not the full archive contract.

**Recovery backup:** use SQLite's online backup API or supported consistent snapshot approach; attachment BLOBs are included by construction and must be verified. Provide user-readable install/upgrade, path, backup, restore and clean reinstall instructions. Local backup media protection is the user's responsibility; export contains personal data and is not encrypted by default.

**Future compatibility proof:** v1 must pass export → fresh v1 import byte/semantic comparison and validate independently readable golden fixtures. V2 must later ship a **tested** importer accepting the frozen v1 format, with explicit migrations and actor mapping. **A v2 import cannot be proven today because v2 does not exist.** Keep a frozen fixture for every released format version.

## 6. Security and privacy without v1 IAM

- HTTP listener **only** on `127.0.0.1`; forbid wildcard bind, unexpected Host and cross-origin writes; no CORS. Browser session uses a cryptographically random per-run access bootstrap token exchanged for an HttpOnly, SameSite cookie and immediately redirected to a clean URL, plus CSRF token and strict Origin checks. This protects against other local browser origins and reduces exposure from other OS users; it is **not** identity management and there is no account/password/role UI.
- Explain remaining trust boundary: processes running as the same OS user can read/write accessible local data. A loopback port alone does **not** restrict access to the launching OS account; bootstrap token mitigates unintended local access, not a fully hostile multi-user host.
- Parameterised SQL, server-side validations, escaped templates, CSP, X-Content-Type-Options, Referrer-Policy, no reflected HTML from imported notes, bounded HTTP bodies and response types.
- App-owned files 0700 directory/0600 data; detect and reject unsafe symlinks and path traversal for archive and export destination; serve attachments from validated DB IDs, not filesystem paths; support safe filename handling and never execute imported content.
- Archive protections: entry count/byte/ratio budgets; no `../`, absolute paths, symlink entries, duplicate ZIP names or untrusted JSON types; integrity-before-import; transaction rollback after corrupt/disk-full interruptions.
- No outbound traffic, telemetry, remote fonts, scripts, automatic update calls, external APIs or model calls. The browser can render local-only pages when Internet is disconnected.
- **Browser bootstrap confidentiality:** The launch/access secret must never be included in browser command-line arguments, a navigated URL (query or fragment), referrer, browser history, diagnostic output or HTTP logs. Deliver one short-lived, single-use bootstrap token through a private OS-user-only local handoff (for example an ephemeral 0600 launcher file) and then exchange it for an HttpOnly, SameSite cookie. The authenticated session expires at process shutdown; deny unauthenticated requests. Any bootstrap exception to the normal Origin rule must be limited to the exact endpoint and require the one-time token. All GET requests must be read-only; state changes require CSRF validation and an allowed origin.
- **Untrusted attachment response:** Original HTML, SVG, PDF and other user/import-supplied files must not be rendered as active same-origin pages. Return untrusted attachments by ID with `Content-Disposition: attachment`, `X-Content-Type-Options: nosniff`, a restrictive content type, and safely encoded filenames. No executable preview of user HTML/SVG; any future safe image thumbnail must be separately re-encoded before inline display.
- **Export file safeguards:** Create archives, recovery copies and CSV outputs with exclusive creation and 0600 owner-only permissions where supported; reject symlink/path traversal, warn when filesystem permissions cannot be enforced, and never overwrite a destination without confirmation. Check space and clean up incomplete temporary outputs. Full and filtered contact-data exports include opt-outs for lawful data portability; marketing recipient lists must always use the consent-filtered campaign export path.
- **Permanent deletion and residual copies:** A confirmed hard deletion runs in a transaction that removes/redacts personal content across indexed records, notes, aliases, recipient PII, activity payloads and attachment bytes without breaking non-identifying historical references. Ensure future exports exclude the purged PII. `secure_delete`/WAL checkpoint and, where practical, controlled vacuuming may reduce local remnants, but do not claim forensic SSD erasure; previous archives/backups require user-managed retirement and should be disclosed.
- **Release dependency gate:** Pin/verify Go modules and the chosen SQLite driver, review supply-chain licences, run a maintained Go vulnerability scan where available, and ensure no runtime CDN, telemetry or surprise outbound dependency is introduced. A scan is evidence, never a security warranty.

- Campaign suppression on every recipient export; consent is not conferred between brands. Audit/change history is honest local history, **not tamper-proof evidence**.

## 7. UI and graphics — small, polished, accessible

- Server-rendered primary views and forms first; semantic HTML, CSS layout, embedded SVG iconography and microillustrations. Keep fonts local/system; polished commercial visual hierarchy with consistent spacing, surfaces, compact but legible list tables, context drawers, quiet colour coding and meaningful empty/loading/error states. Dark and light themes.
- JavaScript only where it improves interaction: search suggestions, incremental updates, drag Kanban, compact drawers; all essential creates/edits and Kanban moves have server-rendered/no-JS equivalents.
- Kanban uses one authoritative transition endpoint; accessible move buttons and announcements; virtualise or paginate long columns rather than rendering thousands of cards; optimistic UI must revert and explain failed writes.
- CSS/SVG charts with table alternative; reduced-motion and keyboard support; avoid heavy canvas loops, heavyweight SPA, network assets and decorative artwork that hurts clarity. Prefer the Garage principle of **high visual quality per byte**, not low visual quality.
- Test at 1280×720, narrow tablet and keyboard-only; include original appearance UAT for contact profiles, opportunity pipeline, campaign planning and the My Day task board.

## 8. Constraint budgets and behaviour at the limit

The following are **proposed targets**, not measured outcomes. Record actual conditions and revise justified thresholds based on evidence; do not invent capacity claims.

| Resource/operation | Normal design target | Stress envelope | Behaviour at limit |
|---|---|---|---|
| First usable view (warm) | p95 ≤500 ms on target Ubuntu laptop | dashboard with 10k contacts, 20k tasks | show usable shell and paginate/defer heavy aggregates; never block on a giant list |
| Indexed CRUD/search | p95 ≤150 ms common writes/search | 100k people, 100+ brands, 1000+ offers | bounded queries and paginated results; visible loading/error, no partial write |
| Cold launch | target ≤2 seconds to listening | large supported DB | visible startup state; fail explicitly if DB/lock/migration invalid |
| Idle CPU / background work | no polling or scheduled worker; target <1% sustained CPU after settling | system resumed after prolonged inactivity | compute due indicators on request; bounded refresh; never spend CPU on dormant monitoring |
| Binary and frontend footprint | record compressed/uncompressed release sizes; target ≤40 MiB executable on AMD64 | packaging and embedded artwork revisions | oversized build triggers explicit simplification review; do not silently remove useful UX |
| Write concurrency | one coordinated writer, bounded requests | multiple browser tabs and rapid submissions | finite busy timeout and visible retry/conflict; no unbounded in-memory queue or lost updates |
| Restore recovery time | record bytes and elapsed time for 1 GiB fixture; provisional ≤3 minutes on reference SSD | multi-GB user archive | show progress and disk-space requirement; never risk the current database to meet a speed target |
| Resident memory | target ≤150 MiB routine | profile at 100k people/100k activities | stream archives and paginate; cap input buffers; no process OOM as accepted failure mode |
| HTTP inputs and files | bounded form body; bounded attachment import (initial suggested 25 MiB each, subject to UAT) | large valid customer dataset | reject with clear reason before mutation; safe configurable limits without brand/product count caps |
| Portable archive | stream and checksum; no whole ZIP in RAM | disk-limited multi-GB archives | check free-space budget and configurable unpacked limits, refuse safely if insufficient; preserve old store |
| Data corruption/disk full | preserve committed records | power interruption mid-export/import | read-only diagnostic/restore when unsafe; never serve partially committed data |
| Missing frontend JavaScript | all core record operations still available | unsupported browser feature | server-rendered form/menus and task-stage change remain usable |
| No Internet | all v1 functions available | offline indefinitely | no degraded experience, because Internet is not a dependency |

Reference fixtures: 10 brands, 1,000 offers, 10k people, 5k leads, 5k opportunities, 100 campaigns, 20k tasks, 100k activities **plus** 100-brand/100k-person stress. Measure load, memory, binary size, query plans and p50/p95 on actual Linux hardware. No artificial limits on logical brand/offer/campaign counts.

## 9. Degradation and recovery contracts

- **Disk full:** fail write atomically with actionable message, remain readable where DB healthy, permit export to another writable destination; no silent truncation.
- **Corrupt import/archive:** reject before live mutation and preserve original DB+attachments; show offending file/field without exposing hidden secrets.
- **Corrupt live database / failed migration:** stop writes and offer verified backup restore or diagnostic export where possible; do not automatically overwrite last good backup.
- **Crash during attachment operation:** a single SQLite transaction guarantees that either BLOB and metadata commit together or neither does; validate hash on retrieval/export.
- **Crash during full restore:** recover using the durable active-generation selector: if promotion was not committed, reopen previous valid DB; if committed, reopen staged verified DB; on corrupted/inconsistent selector fall back to the explicitly verified previous generation with an operator-visible recovery event. Retain last good backup, checkpoint/close WAL/SHM safely and never silently delete a recoverable generation.
- **JavaScript disabled:** forms, search, task stage changes, records and exports remain accessible, with reduced interactive polish.
- **Stalled expensive report:** return paginated/basic table and show query failure; ordinary task/edit flows remain available.

## 10. Verification — design gates vs implementation tests

**Design gates:** PRD traceability, brand/offer/campaign schema invariants, simplification/removal test, resource-budget table, local security threat model, end-to-end decision loop, v1-v2 boundaries and archive format specification. Review evidence in companion assurance report.

**Code/release gates (NOT run or passed yet):** `gofmt`, `go mod tidy` no diff, `go vet ./...`, `go test ./...`, `go test -race ./...`, Linux AMD64 executable run and ARM64 build, smoke tests with network blocked, contact/lead/opportunity/campaign/task end-to-end acceptance, live SQL integrity, security negative tests, fixture round-trip of **every** entity and binary, disk-full/crash injection **before and after active-generation selector promotion**, merge-consent/historical-snapshot preservation, two-tab stale-edit conflict, no-brand first use, fractional quantity/rounding/overflow, DST recurrence, JS-off UI, keyboard accessibility, reference load and stress, browser visual UAT and install/upgrade/restore test. The extended case matrix is in the v0.4.1 assurance report; delta cases and review rationale are in the v0.4.1 final review.

**Security release additions (NOT run):** verify one-time bootstrap token is absent from process listings/URLs/logs; verify forced-download/no-sniff content for malicious attachments; verify restrictive archive/backup/CSV file modes and clear privacy warnings; verify hard-delete redaction across every export and note remaining backups; run dependency/vulnerability scan.

Do not say "zero latency overhead", "security passed" or "migration guaranteed" until the relevant measured tests have passed. Stating that no extra network hops were added is an architectural observation, not a performance measurement.

## 11. Build discipline and publication

Follow the uploaded Garage GitHub Publication Learnings exactly: lock PRD/EDD; agree file layout; implement and test coherent source locally; publish stable repository structure, documents, source, tests and assets in a few verified connector operations; only correct demonstrated CI failures; generate code-derived EDD/comparison from **final source-changing commit**; tag/release only after complete repository and green CI; prefer a click-to-run Linux x86_64 AppImage with desktop metadata and writable data outside the image (plus ARM64 if validated), and put the distributable in GitHub Release, not in source. Browser-only human upload is a **fallback after one failed connector attempt**, with exact paths and verified bytes; never invent staging/transport machinery.

## 12. V2 roadmap and upgrade architecture — no speculative implementation

1. **Multi-user:** v2 selects local-network or hosted deployment and corresponding data/concurrency model, then adds accounts/sessions, users, roles, brand permissions and audit attribution. Migration maps all v1 local activity to initial owner while retaining original records and timestamps. Existing brands are **business classifications, not automatically security tenants**.
2. **Model harness:** separate boundary for optional read-only analysis and proposed writes; per-brand context, data minimisation, action approval, immutable run log in v2, explicit fail-closed on ambiguous/harmful writes. Deterministic domain validation remains authoritative. No model code now.
3. **Third-party integrations:** optional adapters for email, calendar, campaign dispatch, form capture, messaging, accounting, enrichment. V2 defines scoped credentials, rate limits, idempotency, source provenance, retries, audit and data consent when built. No connector runtime or credential table now.
4. **Migration:** freeze and publish `garage-crm-export/1` and golden fixtures with v1. V2's migration test must import all records, attachment BLOBs, custom fields and history, maintain stable IDs and produce a reconciled digest/exception report. No silent loss or generic merge semantics.
5. **No premature extensibility:** today's compact domain functions and data definitions may be refactored when a real second implementation exists. A hypothetical future plugin is insufficient justification for interfaces now.

## 13. Decision

**Architecture: design lock candidate after final Garage review.** The v1 functional scope and v2 migration provisions are retained. Documentary consistency checks and review disposition are in `Garage_CRM_Final_Garage_Constraints_Review_v0.4.1.md`. Implementation, runtime performance, security and v2 importer compatibility remain unverified until actual code and later v2 exist; this is not release certification. The v1 security boundary is one trusted Linux OS account, not a hostile multi-user machine; plaintext data on an unlocked/unprotected disk and copied export files remain exposed to OS-level compromise or anyone given those files.
