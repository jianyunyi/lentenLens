# Remaining lint findings deferred from PR #18

The latest local run on PR #18 head `4b1ac1fd663cccb024b7435b614b9b157df9fdd9` used golangci-lint v2.13.0 and reported 96 findings:

| Linter | Findings | Decision |
| --- | ---: | --- |
| `govet` (`fieldalignment`) | 42 | Deferred: many findings are on persisted or API-facing structs. Reordering fields can change `encoding/json` output order and byte-based digests. Do not auto-reorder without serialization compatibility tests. |
| `gocyclo` | 40 | Deferred: findings are in recovery/apply, capture, sharing, indexing, provider, and CLI paths. Refactoring these only to satisfy a threshold risks changing validation order, error behavior, locking, or fail-closed behavior. Keep the configured threshold; address functions in separately tested feature refactors. |
| `gocritic` (`hugeParam`) | 11 | Deferred: the warnings are performance suggestions. Some signatures are exported, while pointer conversions for private helpers need aliasing review; no signature changes are justified solely for lint. |
| `dupl` | 2 | Deferred: the similar directory-validation routines enforce separate confined storage namespaces and produce distinct errors. Preserve both until a helper can be introduced with tests for creation, missing paths, symlinks, irregular files, and namespace separation. |
| `unparam` | 1 | Deferred: this is in a test-only call-path oracle helper. It is not a runtime defect, and removing the fixture parameter adds little value. |

No linter was disabled, no threshold was relaxed, and no findings were suppressed. These remain reported findings, not passing checks. A dedicated future cleanup should include the compatibility and security tests noted above.

## Validation on this follow-up

`go test ./...` passed on the PR #18 baseline. The workflow YAML files parsed successfully after the workflow changes. The lint run itself completed and reported the 96 findings above; CI may continue to report Code Quality as failing until the deferred work is addressed safely.
