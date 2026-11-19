# Garage CRM — Product Requirements Document

**Design version:** 0.4.4  
**Target build:** Version 1 (single-user, offline Linux)  
**Status:** Locked version 1 design baseline; development implementation underway (acceptance not complete)  
**Date:** 16 November 2026  
**Companion:** `EDD.md` (design v0.4.4)  
**Evidence (non-normative):** `Garage_CRM_Final_Garage_Constraints_Review_v0.4.1.md` (final gate), `Garage_CRM_Research_and_Assurance_v0.4.1.md` (startup benchmark)

## 1. Mission and non-negotiable outcomes

**Mission:** Help one person manage relationships, find and qualify leads, sell multiple products/services across any number of businesses, plan and record marketing campaigns, and complete the next important task — without cloud services or administrative overhead.

- The four equal core domains are **Contacts/Organisations, Leads, Opportunities and Marketing Campaigns**. **Tasks/activities** is a first-class cross-domain workspace with a **Kanban** board.
- The default home screen is **My Day**: due/overdue follow-ups, uncontacted leads, stale open opportunities and campaign actions, with immediate actions. Charts are available but secondary.
- The application works **completely offline** after installation. There are no network integrations, external APIs, telephony, emails sent, tracked opens, background sync, hosted accounts, analytics beacons, LLMs or model harness runtime in v1.
- One Linux OS user, one local Go application and SQLite. No users, roles, organisations-as-tenants, IAM, sign-in/accounts or permission administration in v1. Browser access protection is a technical local safeguard, **not** a user/IAM feature.
- **No artificial application count limit** for businesses/brands, products/offers, campaigns or contacts. Resource use must be bounded and tested; physical machine limits remain real.
- A complete portable export and restore/import is mandatory from the first release, independent of proprietary services, so later v2 migration is possible without losing records or history.
- Competitor knowledge informs outcomes only; nothing in this normative document is a vendor blueprint.

## 2. First-run, navigation and global behaviour

- Launch installed executable (desktop launcher preferred); local browser opens to loopback. First-run setup asks for **nothing beyond optional default brand**. An unbranded global workspace is permitted. Global people, organisations, notes and standalone tasks can be created immediately. The **first brand-scoped** action (lead, opportunity, offer, campaign or brand-specific relationship) uses a one-step inline **Create/select brand** prompt, never a mandatory account/setup wizard. Thereafter the current brand supplies the default; all brand-scoped records always store a valid brand ID.
- Left navigation: My Day, Contacts, Organisations, Leads, Opportunities, Campaigns, Tasks, Reports, Settings. Always-visible current brand filter: **All Brands / Single Brand / saved selection**; tasks and global search respect context with visible scope.
- Global quick create for a contact, lead, opportunity, campaign, task and activity. Quick search across people, companies, deals, leads, campaigns, tasks and manually logged notes (with bounded/paginated results). Keyboard access and a command palette if lightweight.
- Clear distinction between **global person records** and **brand-specific relationships**. Changing brand switches the commercial context, not the person's identity.
- Saved filters/views; predictable sorting and paging; bulk tag/archive and bulk task-date updates with preview and confirmation. Optional custom fields of limited types: text, number, date, boolean, select and multi-select. No scripting, user-defined SQL or automation engine.
- One-click navigation from record to related tasks and a chronological timeline; split list/detail views where helpful. Edits retain context; empty states provide a next action, not an empty chart. Two open browser tabs must not silently overwrite each other: a stale save returns a non-destructive conflict with the current record and reapply/reload choices.
- Optional demonstrator dataset is isolated, clearly marked and removable; must not be mixed invisibly into real records.

## 3. Brands, products and the shared customer universe

- Create, edit, archive, restore, rename and colour-code **unlimited logical brands/business lines**. Each brand has description, default currency, optional local logo and its own pipelines, products, campaigns, contact relationships and reporting.
- Products/services/offers belong to one brand; support software subscriptions, contracting/consulting services, labour supply, applications, physical goods and other types. Name, offer type, description, code/SKU optional, active/archived state, optional indicative price and ISO currency. **No inventory, subscriptions ledger, payments or invoices in v1.**
- One global person or organisation can engage with many brands independently. Store per-brand lifecycle, tags, source and communication preference/permission. A person can be a prospect for one product and an established customer for another, including within the same brand; record **per-offer interest/status** rather than forcing one global lifecycle flag.
- A person can be associated with multiple organisations and roles (including historic affiliations), not just one current company. Organisations may have parent/child connections and many people. An opportunity can have multiple participants with buyer/influencer/champion/other roles.
- Cross-brand audiences always remain **separate brand-scoped campaigns**, with optional common *campaign family label* for consolidated read-only reporting. No cross-brand auto-joining of contacts or reuse of marketing permission.
- All Brands reporting reconciles against per-brand figures; distinct people vs brand relationships are separately labelled. Multiple currencies are never invisibly summed.

## 4. Contacts and organisations

**Core features:** contact and company create/edit/archive/restore; names and roles; multiple email addresses/phones; addresses, websites as stored text, notes, tags, custom fields, preferred channels, important dates; association and relationship histories; quick links to leads, opportunities, campaigns and tasks. Show last actual interaction and next planned contact, clearly labelled.

- Fast duplicate suggestions via normalised email and name/company, but **no automatic merging** or forced uniqueness on shared addresses. Show conflicts, preview, chosen canonical record, related-record moves, undo limits and committed result. **Never loosen global suppression or brand/channel opt-outs as a side effect of merging**; conflicting values use the more restrictive state pending explicit user review. Preserve the original recipients and outcomes in historical campaign snapshots, while resolving current eligibility through canonical identity.
- Locally attached files (documents or images) may be linked to people, companies, leads, opportunities and campaigns, with originals preserved in the portable export. One chronological **manual** activity timeline (with edits recorded in change history): calls, meetings, emails, messages, introductions, research notes, decisions and outcomes. A record can log an interaction and create a follow-up task **in the same save**; channel, date, note and next step all optional except meaningful text.
- Changes to a global name or number propagate to each brand view while brand-specific statuses/consent remain independent. Archive does not erase links or history. Permanent deletion, when explicitly requested, must show a linked-record impact preview and require confirmation; it must not silently cascade into unrelated records.
- Brand-level permission to contact and contact-level do-not-contact status, with a **global suppression overriding brand permission**. These are data and safety controls, not a legal compliance certification.
- **Permanent deletion semantics:** A confirmed permanent deletion must purge personal identifiers, free-text personal content and attached bytes associated with the subject from active business records, indexes and subsequent full/CSV exports; preserve referentially valid, non-identifying tombstones and commercial history only where necessary, and redact affected history/event payloads. A preview must explain what cannot remain (including linked notes) and what non-identifying history remains. The application cannot promise forensic erasure from SSD blocks, previous backups or copies already exported; warn accordingly and explain how to retire old backups. Do not present archival as permanent deletion.


**Acceptance C01-C06:** CRUD and cross-brand identity; multiple-company affiliations; duplicate preview and transactional merge; one-save activity + follow-up; archive and history retention; independent brand/offer lifecycle and suppression.

## 5. Leads

- Manual quick capture from anywhere and CSV import. Capture minimum useful identity (person or organisation or descriptive lead title), brand, source, optional campaign and offer interest; status New / Contacted / Qualified / Disqualified / Converted / Nurture, priority, optional manual qualification score, estimated value/currency, expected time frame, notes, qualification checklist and disqualification reason.
- Lead Inbox list and Kanban, with due/overdue actions, sorting and saved filters; manual touchpoints share the common timeline. Store original source/campaign attribution (including optional **manually entered** source/medium/campaign tags) through conversion.
- A lead can relate to more than one offer. **Convert** into an opportunity with existing or newly created contact, or mark as an established contact without requiring an opportunity. Preserve the lead's ID, qualification and all history. Repeated conversion submissions must not create duplicates.
- Retain disqualified/nurture leads for future campaigns without confusing them with active opportunities. No autonomous lead capture or external enrichment.

**Acceptance L01-L05:** quick capture; lead Inbox/qualification; no-loss conversion; idempotent repeated submission; saved source/offer attribution.

## 6. Opportunities and sales pipelines

- Open multiple opportunities per person or organisation with a brand, a brand-specific pipeline and clear stages. Default stages Prospecting, Discovery, Proposal, Negotiation, Won, Lost; user may rename/add/reorder stages and create additional pipelines when distinct sales motions justify it.
- List and Kanban views, keyboard move-stage alternative, filtered forecast, stage age, stale-deal alerts, lost/won reasons, optional named competitor, forecast close date, actual close date, manual probability and visible next action.
- Opportunity lines can refer to brand offers or a free-text service line with **decimal quantity**, unit amount, line total and currency; define an explicit **manual total override** if required and display the reconciliation. Multiplication and rounding to the currency's minor unit are deterministic, checked for overflow and disclosed. No silent cross-currency sum. Opportunity's reporting currency is explicit and line currencies must match it in v1 (change currency by an explicit user action; no FX conversion).
- Opportunity participants link any number of contacts and their role; related interactions, tasks and campaigns remain visible. Keep **append-only stage transition history** (including stage IDs, names at transition, from/to, timestamp, reason when provided) for later time-in-stage reports and lossless migration.
- One **primary originating campaign** may receive attributed won value; any number of secondary influences may be listed without contributing again to attributed totals. Won value is **closed opportunity value**, not recognised cash revenue.
- Weighting: sum open opportunity amount × explicit probability; handle amounts by currency and never include Won/Lost in open forecasts. Reports disclose date range, scope and denominator.

**Acceptance O01-O07:** stage moves and histories; cross-brand rejection; line-total reconciliation; multi-contact buying groups; provenance; won/lost and reopening; reconciled reports by brand/offer/currency.

## 7. Marketing campaigns — offline planning and manual measurement

- Unlimited campaigns, each explicitly **owned by one brand** and able to target multiple offers of that brand. Campaign family labels can link related but independent brand campaigns without blending their audiences.
- Campaign goal, audience, channel (email, calls, events, social, direct mail, other), schedule, status Draft / Planned / Active / Completed / Cancelled, budget/spend with currency, content/script/subject/call-to-action, manually defined KPI targets, notes, local attachments and campaign task checklist.
- Audience: reusable **dynamic saved segment** (rule-based filter using a bounded set of typed fields/operators, no user SQL/code) and explicitly hand-picked contact list, with preview. On activating, **freeze historical membership and cohort assignment**. Subsequent segment rule edits or contact merges do not rewrite historical membership; explicit additions/removals are tracked as amendments. Current export eligibility is re-evaluated independently.
- Optional A/B variants for manual experiments with defined audience cohorts and separate outcomes. The campaign overview has a simple planned calendar/timeline from its own dates and linked task milestones. Campaign waves/steps can be represented using ordinary linked tasks, **not** an email scheduler, sequencing engine or queue.
- Manually record a recipient's contact status/outcome (including contacted, replied, interested, converted, opted-out, failed/bounced as *manually observed*), touch dates, response, interest, converted lead/opportunity and reason. Unknown remains unknown. No automated open/click/reply/impression measurement, fabricated engagement or unverified causation.
- Recheck suppression **at every recipient-list/mail-merge export**, even after historical audience snapshot has frozen; historical membership remains, but opted-out people are not exported as available recipients. Warn if permission/consent basis is unknown. Copy/text content, CSV recipient exports and simple personalised mail-merge CSV fields only: **no dispatch**.
- Show audience size, eligible/exportable count, touches logged, responses, leads created, opportunities created, closed opportunities, simple manually evidenced conversion, attributed value and spend. Use correct denominators, attribution and currencies.

**Acceptance M01-M08:** brand and offer scoping; saved dynamic audience vs frozen historical snapshot; opt-out safety on later exports; manual multi-channel logging; manual variants; linked tasks/waves; non-fabricated metrics; independent and family-level reports.

## 8. Tasks, activities, action desk and Kanban

- Global task Kanban columns Backlog / To Do / In Progress / Blocked / Done, with list, today/overdue/upcoming, optional date-grid views. Filter across All Brands or per brand, priority, linked item and due date. Preserve manual card ordering within columns.
- Create linked tasks from any lead, person, company, opportunity or campaign or as a standalone global task. Support title, notes, checklist, due date, priority, status, optional recurrence and completion log.
- Keyboard-operable stage changes and a deterministic non-drag control; optimistic UI only with rollback on failed saves. No need to implement a separate task engine for each CRM domain.
- Recurrence uses simple daily/weekly/monthly rules generated on use/after completion with deterministic duplicate prevention. On re-opening after long inactivity, show the missed period but **never backfill a flood of historical tasks**; maintain at most one next outstanding occurrence for a series until it is acted on. No daemon or OS background notifier. Reminders appear **only while the application is open**.
- My Day ranks and explains: overdue/due-today tasks, new leads awaiting contact, open opportunities lacking next task, stalled relationships and upcoming campaign milestones. Thresholds are configurable simple numbers; do not use AI or hidden scoring.

**Acceptance T01-T07:** board/list/date; context filtering; recurrence without duplicates; keyboard use; overdue rules; direct next-action navigation; accurate deterministic alert explanations.

## 9. Reports, search and analytics

- Reports for lead sources and conversion, opportunity stages and histories, open weighted pipeline, closed won/lost, stage ageing, activity volume, tasks, campaign outreach and manually recorded results; per brand, offer, campaign, date range and supported currencies.
- Quick drill-through to underlying records. Counts distinguish people from brand relationships and campaigns from unique individuals. Charts use fast local SVG with accessible text/table equivalents. No chart indicates actual email open/click data.
- Global search, pagination and saved views; export the filtered result as CSV. No arbitrary user-built query language.

**Acceptance R01-R05:** exact reconciliation, currency-safe aggregation, no invented tracking, historical stage reporting, search/filter responsiveness.

## 10. Data ownership, import/export, recovery

- CSV import with mapping, dry-run preview, duplicate handling, validation and clear failure report; CSV export by entity or filtered result. Spreadsheet-safe CSV output must not destroy the separate lossless archive format.
- **Full portable export/import**: one documented versioned local archive containing every persisted business record and stable ID; brand/offer relationship; all people/organisations/affiliations; leads; opportunities, lines and stage history; campaigns, content, variants, audiences/snapshots/outcomes, campaign-family metadata; notes/activities/tasks; custom fields/views; app preferences; original file attachments and checksums; and locally recorded change history. Include empty/null distinctions and timestamps. No silently omitted entity classes. Attached file bytes reside in the same authoritative database in v1, not an untracked folder.
- Full import into a **new installation** must be lossless, support a precise preview, reject unsupported fields/versions instead of dropping them, and leave any existing database untouched on failure. Importing into an existing populated installation may use **replace after explicit confirmation**, but general merge-import of full archives is not required in v1.
- Separate on-demand SQLite backup/restore for disaster recovery; local safety backup before replacement, integrity tests and instructions for copying the portable archive off the device. No background sync or cloud backup. Permit an explicit safe choice of local data location on first run or during a validated move.
- **Sensitive exports:** Portable archives, database backups and ordinary CSVs contain personal data and are not encrypted by the application in v1. Create and atomically finalise files with restrictive owner-only permissions where supported, never follow unsafe symlinks, never overwrite without confirmation, and warn prominently before exporting to shared/removable storage. Recommend OS full-disk encryption or suitably protected destination. General contact CSV export may include suppressed people for data ownership; it must be labelled **not a campaign recipient list**. Only consent-filtered campaign recipient exports may be used as outreach lists.

- v2 importer must be able to read this v1 archive through an explicit compatibility process; only v1 round-trip and independent format validation can be proven before v2 exists.

**Acceptance D01-D09:** complete field/entity and binary round-trip; stable IDs and historical events; portable schema; corruption/attack rejection; interrupted import/restore rollback; CSV preview; upgrade from fixture; clean install; documented recovery.

## 11. Design quality and usability

- Original, polished, commercially credible UI with warm-neutral or quiet dark palette, coherent typography, clear cards, restrained accents, lucid empty states, low visual noise, compact tables and carefully designed illustration/icons where they materially aid comprehension.
- Build visuals using embedded SVG, CSS and system fonts; limit animation to meaningful transition and acknowledge `prefers-reduced-motion`; no external assets or image-heavy framework.
- Responsive layout for 1280×720 laptop as baseline, usable at narrower widths. Accessibility: focus order, labelled controls, contrast, non-colour indicators, semantic tables, keyboard Kanban and screen-reader text for charts.
- Measured usability scenarios: quick-create person/lead and due task without a setup wizard; record interaction + follow-up in one save; navigate My Day → overdue item → resolution; create campaign/segment/activate/export; switch brands while preserving correct context.
- Visual polish is a **product acceptance gate**, but not grounds for increasing runtime complexity or sacrificing speed/security.

## 12. Version boundaries

### Version 1 — mandatory
All sections 1–11, single-user offline application, all four business domains, Kanban, brands/products/campaigns, complete portability, security and tests. No artificial limits to logical entities.

### Version 2 — planned, **not implemented or configured in v1**
- Optional multiple users, identity, sessions, roles, brand-scoped access, actor-attributed audit, simultaneous edits and collaboration **after** deciding local-server/cloud deployment and database concurrency architecture.
- Optional model harness operating through explicit, scoped read/propose/approve business actions; all sensitive changes need review, audit and brand-context minimisation. No model-integrated decision authority for critical business rules.
- Optional external adapters for email/calendar, lead forms, campaign delivery, messaging, accounting, enrichment and other CRM providers, with user consent, credential protection and observable failures.
- Optional native mobile/cloud deployment and richer automation only if a demonstrated need justifies them.
- **Compatibility prerequisite:** v2 must import a v1 full archive and migrate all IDs and historical records, map legacy local actor to an initial owner, and preserve attribution and consent. v1 contains no user/role/connector/model scaffolding.

## 13. Explicitly excluded from version 1

User accounts and permissions; team assignment; remote hosting; sync; external email sending or reading; external calendar, social, telephony or marketing integrations; external enrichment; webhooks; external/public APIs; hosted forms; LLM runtime; agent workflows; automatic outreach or click tracking; no-code automation engine; arbitrary user SQL/script; payment, invoice and inventory systems; customer support ticketing; real-time collaborative editing; cloud backup; automatic software updates.

## 14. Product-release decision

v1 product scope is complete only when all numbered acceptance criteria are implemented and executed with reproducible evidence. Requirements are approved by design review; **runtime compliance cannot be claimed before a build exists**. Do not silently reclassify missing functionality as v2 to simplify implementation.

## Amendment v0.4.3 — My Day cooling detector

- The existing stale-opportunity and stalled-relationship requirements are made explicit as a 14-day read-only **Cooling — no next action** dashboard alert for live opportunities and brand-linked prospects with no open next task.
- Qualifying deal touch: later of opportunity update and latest linked activity. Qualifying prospect touch: later of contact creation and latest contact activity in the corresponding brand.
- Suppress deal alerts for closed/archived opportunities, and contact alerts for archived contacts and completed/non-prospect lifecycles; a live deal takes precedence over its primary contact in the same brand.
- Suppress alerts while a matching non-completed, non-archived task exists; completed or archived tasks must not suppress. Honours All Brands and selected-brand filter, uses direct record links and displays days quiet.
- Beta threshold fixed at 14 days. Configuration remains an outstanding wider v1 requirement.
- Acceptance includes cross-brand activity isolation, record exclusion, task-state transitions, activity recency, HTML escaping, bounded results and visible count. No new persistence, background scheduler or outbound connection.

## Amendment v0.4.4 — Save reliability, controlled exit and durable upgrades (19 November 2026)

This is a controlled additive amendment to the locked v0.4.3 Version 1 requirements. All existing requirements and acceptance gates remain in force.

- **Save reliability:** A legitimately authenticated local browser form submission must persist and be visible after navigation, Quit, and reopening. An opaque or missing browser Origin is not sufficient reason to reject a valid request when the same user has the private session cookie and unpredictable per-run CSRF token; reject contradictory cross-site Origin, Referer and Fetch Metadata signals. No arbitrary cross-origin trust and no permissive CORS.
- **Quit application:** A visible and keyboard-accessible Quit Garage CRM control must request a graceful termination of the local HTTP listener, allow in-flight operations to finish, close SQLite safely, release the process lock and show a useful browser confirmation. Closing a browser tab alone must not terminate the application. No unauthorised POST or GET may shut the service down.
- **Persistent desktop deployment:** Installing or upgrading the Ubuntu package must not use the Downloads directory as an application or data root. The executable and launcher use standard OS paths; data stays in the XDG Garage CRM directory unless explicitly configured otherwise. Upgrades must not overwrite existing data.
- **Acceptance S01:** Exercise the actual HTTP bootstrap, session, form save, missing-CSRF rejection, Quit and database reopen on a real loopback listener using a disposable database. Exercise explicit cross-site and contradictory header cases. Native Ubuntu browser/UAT and upgrade verification are separate final gates; automated tests alone cannot claim that UAT has passed.
- **Version clarity:** Design version (0.4.4), development build version (0.1.3-dev), Debian package version (0.1.3~dev) and prerelease tag have distinct purposes. The single source of application build version is `VERSION`, passed to the compiled binary and used for packaging; do not claim full Version 1 acceptance.
