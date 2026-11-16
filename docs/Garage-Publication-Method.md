# Garage GitHub Publication Learnings

**Source:** Astro Answers V0.1.0 working session  
**Purpose:** Capture practical rules for how ChatGPT should publish Garage work to GitHub, and when it should stop using connectors and instead create a file for Jonathan to upload through the GitHub web interface.

---

## 1. Governing rule

> **Do not let publication plumbing become more complicated than the artefact being published.**

The source, tests and product are the work. Git transport is not.

If the publication method starts consuming significant time, creating temporary branches, archive reconstruction, staging mechanisms or repeated retries, stop. Return to the simplest reliable path.

---

## 2. Default publication order

For a Garage repository, publish in this order:

1. Repository structure and lightweight text files.
2. PRD, EDD, README and engineering notes.
3. Source files.
4. Tests.
5. Binary assets.
6. CI workflow.
7. Run CI and fix only what the gates expose.
8. Generate code-derived evidence after the final source-changing commit.
9. Tag/release only after the repository is complete and CI is green.
10. Attach distributable binaries such as AppImages to the GitHub Release, not the source tree.

Do not create the release tag early.

---

## 3. What ChatGPT should upload directly

Use the GitHub connector directly when the operation is straightforward and observable.

Good candidates:

- small and medium UTF-8 text files;
- README, PRD, EDD and Markdown documentation;
- `.gitignore`;
- `VERSION`;
- `CHANGELOG.md`;
- licences;
- Go source files that fit comfortably through the connector;
- Go module files;
- CI workflows;
- small repository housekeeping changes;
- generated engineering evidence;
- deletion of temporary placeholder files.

After each meaningful group, verify what actually landed in GitHub.

Do not infer success from an attempted call.

---

## 4. When to stop using the connector

Stop connector experimentation when any of these occurs:

- a file repeatedly stalls or truncates;
- a binary upload becomes awkward;
- the tool requires reconstructing an archive inside GitHub;
- temporary assembly branches or chunking schemes are being invented;
- the same transport problem has been retried more than once;
- publication is taking longer than the engineering change;
- the connector can technically do the job but only through a much more complex mechanism than a human browser upload.

At that point:

> **Create the exact file for Jonathan to download and give browser-only upload instructions.**

Do not invent another transport layer.

---

## 5. What file to create for Jonathan

Create the smallest artefact that solves the problem.

### One difficult source file

Provide that exact file.

Example:

```text
main.go
```

Tell Jonathan exactly which GitHub directory to open and upload it into.

### Several binary assets

Create a ZIP containing only the required files if that genuinely makes selection easier.

Do not include scripts, staging files, source trees or unrelated documentation.

If GitHub requires the files individually, explain that the ZIP is only a convenient local container and identify the exact files to select after extraction.

### Finished application

Provide the finished distributable directly.

Examples:

```text
AstroAnswers-x86_64.AppImage
```

The release binary should then be attached to the GitHub Release rather than committed into the Git repository.

---

## 6. Browser-only means browser-only

Jonathan uses the GitHub web interface.

Do not assume:

- local Git;
- GitHub CLI;
- shell scripts;
- `git push`;
- terminal access;
- command-line archive tooling.

If a workflow requires any of those, it is the wrong handoff unless Jonathan explicitly asks for it.

Instructions should describe the actual GitHub browser path:

```text
Repository
→ target directory
→ Add file
→ Upload files
→ select exact files
→ Commit directly to main
```

---

## 7. Do not rely on folder upload

Do not tell Jonathan simply to “upload the folder”.

Browser behaviour around directory selection and path preservation is inconsistent.

Instead:

1. ensure the target GitHub directory already exists;
2. if necessary, create it with a temporary `.keep` file;
3. tell Jonathan to open that exact directory in GitHub;
4. upload the files themselves;
5. verify the upload;
6. remove `.keep`.

This worked cleanly for:

```text
cmd/astroanswers/main.go
cmd/astroanswers/assets/*
```

---

## 8. Verify browser uploads byte-for-byte

After Jonathan uploads files:

1. inspect the actual GitHub paths;
2. confirm every expected filename exists;
3. compare Git blob hashes against the authoritative local files where possible;
4. only then remove placeholders or proceed to release.

Do not treat “uploaded” as sufficient evidence if exact source fidelity matters.

---

## 9. CI is the publication gate

Do not declare the repository finished merely because all files exist.

Run the actual Garage CI gates.

For Astro Answers these were:

```text
go mod tidy with no diff
gofmt cleanliness
go vet ./...
go test -race ./...
Linux/amd64 build
Linux/arm64 build
core acceptance tests
```

When CI fails:

- read the failing step;
- make the smallest correction;
- do not redesign unrelated code;
- rerun the gate.

Examples from Astro Answers:

- `go.mod` / `go.sum` failed the module contract → apply the exact `go mod tidy` result.
- `game_test.go` failed formatting → apply `gofmt`; do not change test behaviour.

CI failure is useful evidence, not a reason to introduce new machinery.

---

## 10. Preserve the known-good test and delivery path

A key failure in the Astro Answers session was changing a working Linux packaging path while product changes were still being made.

Rule:

> **Do not modify a proven test or delivery harness while simultaneously changing the system under test unless the harness itself needs to change.**

If the application is changing, hold the harness stable.

If the harness must change, isolate and validate that change independently.

---

## 11. Do not overreact to tool output truncation

Connector output may be truncated for display even when the underlying GitHub operation succeeded.

Before inventing a workaround:

1. inspect the repository;
2. confirm whether the file exists;
3. verify its hash or contents.

Do not assume truncated tool output equals failed upload.

This mistake caused unnecessary detours during Astro Answers publication.

---

## 12. Avoid temporary publication architecture

Do not create:

- temporary reconstruction branches;
- archive chunk assemblers;
- base64 staging trees;
- Git-side unpacking workflows;
- complex publication scripts;

unless there is no simpler supported path.

For a small Garage application, these are almost always evidence that the publication method has become the problem.

---

## 13. Release discipline

The release sequence should be:

```text
final source
    ↓
CI green
    ↓
generated EDD / comparison pinned to final source commit
    ↓
CI remains green
    ↓
release tag
    ↓
release binary
```

Do not tag an incomplete repository.

If a tag is created prematurely, delete or replace it before calling the release complete.

---

## 14. Generated evidence must point to the real final source

`generated/EDD.md` and `generated/COMPARISON.md` should reference the actual final source-changing commit.

Documentation-only commits may come afterward.

The evidence commit should not accidentally point to:

- a pre-format commit;
- a pre-`go mod tidy` commit;
- a source tree missing assets;
- a partial upload.

Pin it only after source, tests, assets and module metadata are final.

---

## 15. Application packaging

For a desktop application, prefer a release artefact appropriate to the user.

For Astro Answers:

```text
AstroAnswers-x86_64.AppImage
```

The AppImage workflow:

- builds from the final GitHub source;
- embeds application metadata and icon;
- keeps `Terminal=false`;
- stores writable game data outside the AppImage;
- verifies the AppImage can be extracted;
- uploads it as a GitHub Actions artefact.

The AppImage itself belongs on the GitHub Release, not in ordinary source history.

---

## 16. Web-user handoff standard

If Jonathan must perform an upload, the response must contain:

1. a direct download link to the exact file;
2. the exact GitHub directory or release page;
3. the exact filenames to select;
4. the exact commit or release action;
5. a clear statement of what **not** to upload;
6. no alternate methods unless the primary method fails.

The goal is that the handoff can be executed without interpretation.

---

## 17. Failure escalation rule

Use this simple decision tree:

```text
Can ChatGPT upload it directly and verify it?
        |
       yes
        |
     upload it
        |
      verify
```

If no:

```text
Can Jonathan upload the exact file through GitHub web UI?
        |
       yes
        |
 create exact file
        |
 give exact browser steps
        |
 verify after upload
```

Only if both fail should a new mechanism be considered.

---

## 18. Behavioural lesson

When a publication method stalls, do not keep narrating increasingly clever fixes.

Say what is actually wrong:

```text
The product is fine.
The transport is the problem.
I am stopping the transport experiment.
```

Then switch immediately to the simplest reliable route.

Progress updates should report completed facts:

- “EDD uploaded.”
- “Assets verified.”
- “CI module gate passed.”
- “AppImage built and extracted successfully.”

Avoid updates that merely promise another publication strategy.

---

## 19. Garage interpretation

The same engineering philosophy applies to the development process itself:

> **Does every piece of machinery remove more complexity than it introduces?**

That question applies to:

- architecture;
- test harnesses;
- CI;
- deployment;
- Git publication;
- packaging;
- ChatGPT tool use.

A clever publication mechanism that needs explanation has probably already failed this test.

---

## 20. Final operating rule

For future Garage work:

> **Build rigorously. Test thoroughly. Publish boringly.**

Use the connector for straightforward repository operations.

When the connector stops being straightforward, stop.

Create the exact artefact Jonathan needs, let him perform the smallest possible browser upload, verify it, and continue.

No rabbit holes.
