#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-"${TMPDIR:-/tmp}/callpath-oracle-metrics.json"}
case "$out" in
  /*) ;;
  *) out="$PWD/$out" ;;
esac

# Keep the benchmark deterministic and model/API-free.  Preserve only the
# explicit oracle output path plus Go's local build controls.
env -i \
  HOME="${HOME:-/tmp}" \
  PATH="${PATH:-/usr/bin:/bin}" \
  GOPATH="${GOPATH:-}" \
  GOCACHE="${GOCACHE:-}" \
  GOMODCACHE="${GOMODCACHE:-}" \
  GOPROXY=off \
  GOSUMDB=off \
  GOTOOLCHAIN=local \
  CALLPATH_ORACLE_JSON="$out" \
  go test "$root/internal/app" -run '^TestCallPathOracleBenchmark$' -count=1
printf '%s\n' "$out"
