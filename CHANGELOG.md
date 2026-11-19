# Changelog

## 0.1.3-dev — Save, Quit and engineering consolidation (19 November 2026)

- Accept authenticated, CSRF-validated local form submissions when the browser omits or obscures Origin, while continuing to reject contradictory cross-site provenance.
- Add Quit Garage CRM to the sidebar, accessible via a POST protected by the normal session and CSRF checks.
- Wait for graceful HTTP shutdown to complete before database close and process-lock release.
- Add real-loopback save/Quit/reopen integration coverage and negative provenance tests.
- Version the executable, UI and Debian package from a single `VERSION` file; install in standard OS paths and retain persistent SQLite data separately.
- Align PRD/EDD v0.4.4, build-status and release records with implementation evidence; preserve outstanding full-v1 acceptance gates.
- Consolidate prior fragmented 19 November changes into one cohesive implementation commit; preserve the old head as a backup branch.

## 0.1.0-dev — Cooling detector enhancement (17 November 2026)

My Day surfaces quiet open deals and brand-linked prospects without outstanding follow-up tasks, with a fixed 14-day threshold. Adds brand isolation, direct links and workflow/UI regression tests. Still a development build; outstanding Version 1 acceptance gates are unchanged.


## 0.1.0-dev — 16 November 2026

Initial working development preview of Garage CRM. Go/SQLite backend, offline browser interface, main entity editors, initial lead/opportunity/campaign/task workflows, scoped campaign recipient export, attachment handling, local-access security boundary, portable data archive and recovery commands.

**Not a v1 release:** the complete locked PRD v0.4.2 acceptance matrix, real Ubuntu visual UAT, full security gates, packaging and CI evidence remain outstanding. See `docs/BUILD-STATUS.md`.
