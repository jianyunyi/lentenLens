# Remaining lint findings deferred from PR #18

The latest local run on PR #18 head `4b1ac1fd663cccb024b7435b614b9b157df9fdd9` used golangci-lint v2.13.0 and reported 96 findings. This follow-up removes the one test-only `unparam` finding; the other 95 baseline findings remain deferred:

| Linter | Baseline findings | Decision |
| --- | ---: | --- |
| `govet` (`fieldalignment`) | 42 | Deferred: many findings are on persisted or API-facing structs. Reordering fields can change `encoding/json` output order and byte-based digests. Do not auto-reorder without serialization compatibility tests. |
| `gocyclo` | 40 | Deferred: findings are in recovery/apply, capture, sharing, indexing, provider, and CLI paths. Refactoring these only to satisfy a threshold risks changing validation order, error behavior, locking, or fail-closed behavior. Keep the configured threshold; address functions in separately tested feature refactors. |
| `gocritic` (`hugeParam`) | 11 | Deferred: the warnings are performance suggestions. Some signatures are exported, while pointer conversions for private helpers need aliasing review; no signature changes are justified solely for lint. |
| `dupl` | 2 | Deferred: the similar directory-validation routines enforce separate confined storage namespaces and produce distinct errors. Preserve both until a helper can be introduced with tests for creation, missing paths, symlinks, irregular files, and namespace separation. |
| `unparam` | 1 | Fixed here: all call-path oracle fixtures used line 2. The helper now keeps that fixture invariant internally; the expected oracle data is unchanged. |

No linter was disabled, no threshold was relaxed, and no findings were suppressed. The 95 remaining baseline findings are still reported, not passing checks. A dedicated future cleanup should include the compatibility and security tests noted above.

## Validation on this follow-up

`go test ./...` passed on the PR #18 baseline. After the test-only fixture change, `go test ./internal/app -run '^TestCallPathOracleBenchmark$' -count=1` passed. The workflow YAML files parsed successfully after the workflow changes. The lint run was on the PR #18 baseline before the test-helper cleanup and was not rerun afterward; the remaining 95 findings should be rechecked by CI. Code Quality may continue to fail until the deferred work is addressed safely.
