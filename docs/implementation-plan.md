# IntentLens initial implementation plan

> Execution: test-driven implementation in this session, with an isolated implementation agent and independent review. Product scope: ../intentlens-design-v0.3.md. This plan implements the first runnable M0/M1 slice; it does not claim completion of M2–M4.

**Goal:** Claude Code round capture, Go function evidence and direct-call navigation, local history, opt-in model explanation with claim checking and budget limits, and a VS Code reading/asking workflow.

**Architecture:** A Go executable exposes JSON commands, invoked by Claude hooks and a small VS Code extension. Local records live outside the target repository in a per-user application directory. No daemon, HTTP listening port, database or frontend framework is needed for this slice.

**Tech stack:** Go standard library; Node JavaScript VS Code extension; Node built-in test runner. Tests use temporary directories and local fake model servers, never external paid calls.

## 1. Capture and navigation

- [x] Add tests for baseline net changes, reverted changes, exclusions, symlinks, oversize files, historical evidence and unsupported call targets; observe failure.
- [x] Implement `internal/capture` and `internal/codeindex` with content versions and explicit incomplete states.
- [x] Run package tests. Review requirements, then correctness/privacy before integration.

Capture API: `Snapshot(root string) (Snapshot,error)`, `Diff(before,after Snapshot) []Change`. Export JSON-tagged data. Honor `.intentignore` before content reads; document supported patterns. Default exclude `.git`, `.intent`, `.env*`, vendor/node_modules and private keys. Limit text files and aggregate bytes; report omitted paths. Never follow symlinks. No transcript ingestion in this slice: raw prompt/transcript privacy cannot be guaranteed by path filters.

Index API: `Build(files map[string]string) Index`. Index functions and source snippets using Go parser/types where possible; errors and unresolved calls explicit. No compilation or dependency downloads during indexing.

## 2. Store, CLI and hook adapter

- [x] Write failing integration tests using project and state temp directories.
- [x] Implement `internal/app`: root identity, atomic local records, per-project mutation lock, start/stop with session matching, list/detail/inspect/status commands.
- [x] Implement `cmd/intentlens`: JSON stdin/stdout command protocol and hook dispatch. Hook failures return success to Claude but preserve local diagnostics. No raw prompt/transcript persistence.
- [x] Test duplicate Stop, missing baseline, overlapping sessions, policy changes and outside-root references.

CLI protocol: `intentlens <command> --root <absolute-project>`, optional `--state <outside-root-dir>` for tests. Additional operation fields are one JSON object on stdin. Commands return one JSON object; errors are nonzero. `hook` accepts Claude input and remains fail-open for the coding flow.

## 3. Explanation and budget

- [x] Test disabled provider, reserve-before-call, malformed/unsupported claims and review mapping with a local HTTP fixture.
- [x] Implement explicit project consent configuration and Anthropic Messages transport with credentials only from environment, no outbound calls until enabled.
- [x] Generate claims bound to evidence IDs; independent support check for each claim; return uncertain status when checking fails. Store history and code hashes, calculate stale state when reading.
- [x] Include ask operation, per-project token cap, pause, actual usage and conservative reservation; document fees as unavailable until configured.

## 4. VS Code extension

- [x] Test JSON child-process bridge and rendering helpers with Node built-in tests.
- [x] Implement trusted-workspace commands, Go CodeLens, round tree, escaped webview, evidence navigation, ask, model enable/pause and status.
- [x] Generate hook configuration without overwriting existing project settings. Require project enable action before capture; no installation into other repos in this implementation session.
- [x] Build executable and package extension, then exercise CLI with simulated official hook payloads.

## 5. Delivery and review

- [x] Run `go test ./...`, `go vet ./...`, extension tests and packaging.
- [x] Run deterministic end-to-end capture/inspect simulation and fake provider explanation; record actual results.
- [x] Review privacy, stale state, budget enforcement, hook error behavior, workspace trust and UI escaping.
- [x] Document setup, supported ignore syntax, local state location, incomplete capabilities and remaining M2–M4 work. Live Claude/VS Code interaction and paid API verification are distinct from simulation.

## Deferred product stages

M2: shared versions, preview/confirmation, claim correction and dependency invalidation. M3: versioned exercises, isolated workspace, execution consent, fixed-test scoring, application preview. M4: real-user trial and full quality corpus. These remain required by v0.3 and are not placeholders represented as working features.

## Delivery status (2026-10-08)
Initial runnable slice is built and packaged. Go: 44 pass, 1 skip; extension: 9 pass; CLI smoke and actual VS Code Extension Host passed. See verification.md for limitations. No remote publication or Git commit. M2–M4 remain unimplemented, and real Claude/API validation remains outstanding.
