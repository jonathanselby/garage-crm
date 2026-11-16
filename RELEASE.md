# Garage CRM — Release Record

**No stable release or tag has been authorised.**

Current implementation: `0.1.0-dev` development preview.

Release only after:
1. All mandatory PRD v0.4.2 scenarios implemented and validated.
2. `go mod tidy` unchanged, `gofmt`, `go vet`, full Go race tests and security negative gates pass.
3. Native Linux x86_64 installation and UI/keyboard visual acceptance.
4. Portable archive round-trip, interrupted restore, large datasets, backup, PII deletion tests.
5. GitHub Actions green on final source-changing commit; code-derived `generated/EDD.md` and `generated/COMPARISON.md` pinned to it.
6. Completed release artifact, correct icon and desktop metadata, distributed via GitHub Release (not source history).

Do not claim release acceptance on the basis of documentation or partial tests.
