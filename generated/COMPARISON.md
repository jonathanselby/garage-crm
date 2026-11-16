# Garage CRM — Locked specification vs implemented source

**Evidence anchored to source commit:** `99da8cc802de11beea90e955c07b0422cbdf2220`  
**Design baseline:** v0.4.2, development build only. Status is based on code inspection and selected automated tests, not complete product UAT.

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
| Safe recovery/backup and upgrades | SQLite online backup, migration v1 | Core implemented; disaster UX/large-scale tests outstanding |
| Security | Loopback, session token, strict web checks and bounded SQL/file inputs | Selected tests passed; no security certification |
| Lightweight polish and accessibility | Embedded original CSS/SVG, server-side templates and keyboard forms | Implemented visually by design; real browser UAT outstanding |
| V2 support | Stable IDs, schema version and portable archive | Direction preserved, actual V2 importer cannot be proven |

**Release decision: NOT READY FOR STABLE VERSION 1.** Do not tag or issue stable binaries before acceptance gaps are eliminated and final tests pass.
