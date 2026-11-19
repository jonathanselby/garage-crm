# Garage CRM — Release and publication record

**Product:** Garage CRM — single-user, offline-first Go/SQLite mini-monolith
**Locked design contract:** PRD and EDD v0.4.4 (additive changes to v0.4.3)
**Current development build:** `0.1.3-dev` (Debian package `0.1.3~dev`)
**Status:** Development/early beta. **No stable v1 release authorised.**

## Published historic prerelease

GitHub `v0.0.1` is an actual prerelease published 16–17 November 2026 and points to `eb7a7e079376e91d81f26522d6d5a843dee40c33`. Preserve the original immutable tag; it has no attached distributable assets. It is not evidence that the current build or all Version 1 requirements have been accepted.

## Current corrective development build

Scope: fix form-save Origin edge cases without disabling CSRF/session enforcement; add user-controlled Quit with graceful shutdown; retain local persistence; add real-listener regression tests; consolidate and verify Git history, specifications and package version.

Application release binary and CI artefacts must derive from the final tested source commit. The Ubuntu `.deb` installs to `/usr/bin` and `/usr/share/applications`, never into the Downloads directory; CRM data remains in its existing XDG data root.

**Not yet proven:** native Ubuntu GUI UAT, an installed-package upgrade retaining *this user's* data, complete security assurance, all v1 PRD acceptance cases, large-data performance and staged in-place restore. Do not use a stable release tag. When native UAT succeeds, an appropriately labelled `v0.1.3` prerelease can be created manually with the CI-built installer attached; release notes must retain these limitations.

## Publication requirements

1. Confirm clean candidate source and reproducible development tests, including AMD64 and ARM64 CI.
2. Preserve original `v0.0.1` and backup `main` prior to rewriting history. Reconstruct the correction from the old baseline as **one source/design commit**, not many micro-commits.
3. Pin `generated/EDD.md` and `generated/COMPARISON.md` to that exact source commit in **one documentation-only evidence commit**.
4. Publish the verified candidate to `main` with a force-with-lease update only if no other writer has moved the branch; verify repository tree and CI.
5. No release artefacts in source history. Place the `.deb` on a prerelease only after final desktop validation; mark beta, not stable.
6. Full Version 1 release is blocked until all PRD acceptance scenarios and security, install, export/import and recovery gates pass.

Do not infer deployment success from an earlier CI build or from green tests alone.
