# Garage CRM — Locked specification vs implemented source

**Evidence anchored to source commit:** `b5c1b3cb00a8d6d64704ff272c0421f8da5edf46`  
**Design baseline:** v0.4.4; application 0.1.3-dev, development build only. GitHub [CI run 44451947256](https://github.com/jonathanselby/garage-crm/actions/runs/44451947256) passed for the exact source commit. Status reflects the inspected code and automated tests, not complete desktop UAT.

| Requirement family | Observed implementation | Disposition |
|---|---|---|
| One offline Go + SQLite process | Executable, loopback web app, one SQLite DB | Implemented/tested |
| Single user without IAM/integrations/AI | No external providers or user account infrastructure | Implemented/tested |
| CRUD brands, offers, people, organisations | Models, forms, archive and relations | Core implemented; full UX partial |
| Unlimited brands/products/campaigns | No hard-coded logical record limits | Design implemented; scale untested |
| Contact multi-brand, affiliations, multiple email/phone | Linked records and forms | Core implemented/tested |
| Contact duplicate merge/consent preservation | Aliases, canonical remapping, restrictive consent; preview | Core implemented/tested; advanced conflict UI partial |
| Permanent personal redaction | Transactionally redacts linked PII and BLOBs | Tested representative cases; exhaustive proof outstanding |
| Lead capture, stages, conversion and original history | CRUD, stage events, idempotent opportunity conversion | Core implemented/tested; checklist/inbox views partial |
| Opportunities, participants, line items, stages, currencies | DB, default stages, amount line editor, stage event history | Core implemented; per-brand pipeline configuration partial |
| Marketing campaigns, variants, audiences, outcomes | Manual template and snapshot, opt-out-safe CSV, manual outcome | Core implemented/tested; saved segment criteria, A/B cohorts, full reports partial |
| Tasks, Kanban and My Day | Server-rendered five-column board, tasks, activity+follow-up | Core implemented; manual ordering/date-grid and nuanced reminders partial |
| Reports and search | Basic dashboard and per-currency stage counts | Partial; full reporting and global search outstanding |
| Custom fields, saved views, bulk editing | Schema definitions only | Not delivered through UI |
| CSV import and general CSV export | Campaign recipient CSV only | Outstanding |
| Portable export/import | Full 34-table ZIP including BLOBs, safe clean-target import | Implemented/tested for fixtures; in-place atomic restore and streaming scale outstanding |
| Safe recovery/backup and upgrades | SQLite online backup, migration v1 | Core implemented; installed-package upgrade/disaster UX/large-scale tests outstanding |
| Security | Loopback, one-use bootstrap/session, per-run CSRF, explicit cross-site provenance rejection and bounded SQL/file inputs | Selected tests passed; no security certification |
| Lightweight polish and accessibility | Embedded original CSS/SVG, server-side templates and keyboard forms | Implemented visually by design; real browser UAT outstanding |
| V2 support | Stable IDs, schema version and portable archive | Direction preserved, actual V2 importer cannot be proven |

## Amendment 0.4.4 delta — inspection of source commit `b5c1b3cb00a8d6d64704ff272c0421f8da5edf46`

| 19 November correction | Actual source and verification | Disposition |
|---|---|---|
| Legitimate POST saving with absent/opaque Origin | `internal/web/web.go` provenance check plus session and CSRF validation; real-server `main_test.go` persists a person through `Origin: null` | Implemented and automated-test passed; native browser UAT pending |
| Protection against foreign-origin submissions | Origin, Referer and Fetch Metadata negative tests plus CSRF requirement | Tested representative hostile requests; full security assurance pending |
| In-app Quit button | Sidebar template protected `POST /quit` with confirmation | Implemented/tested |
| Graceful shutdown and lock release | `serve` waits on `stopped` before deferred DB close; real-listener Quit/reopen test | Automated-test passed; long-running in-flight write/stress UAT pending |
| Persistent Ubuntu installer | Native x86_64 `.deb` metadata, binary, icon and desktop entry in standard paths; no database in installer | CI builds package; native installed upgrade UAT pending |
| Coherent version | `VERSION` -> build ldflags -> runtime `AppVersion` -> Debian package conversion | Build version checks passed on AMD64/ARM64 |
| PRD/EDD amendment | Locked additive v0.4.4 amendment; all earlier v0.4.3 requirements remain | Documented; missing v1 requirements remain blockers |

**Release decision: NOT READY FOR STABLE VERSION 1.** Do not tag or issue stable binaries before acceptance gaps are eliminated and final tests pass.
