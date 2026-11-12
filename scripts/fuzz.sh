#!/usr/bin/env bash
set -euo pipefail

# Discover targets so new tests are automatically included in nightly runs.
# Listing/compilation errors must fail the job, not silently omit a package.
#
# FUZZ_LOG_DIR, when set, receives each target's output instead of stdout, and
# FUZZ_LOG_DIR/failed.txt lists the failing targets. CI uses this to keep
# crashing inputs, which may describe an exploitable bug, out of public logs.
# Every target runs even after an earlier one fails.
if [ -n "${FUZZ_LOG_DIR:-}" ]; then
  mkdir -p "$FUZZ_LOG_DIR"
  : >"$FUZZ_LOG_DIR/failed.txt"
fi

# fuzztime_race reports whether log $1 shows only the Go toolchain race at the
# -fuzztime deadline: the fuzz coordinator sees its timeout context finish
# before the derived worker context is cancelled, and reports the deadline
# itself as the failure. Every log line must be fuzz progress or part of that
# exact failure; any other line, including crash minimization, still fails.
fuzztime_race() {
  local log=$1 target=$2
  grep -qxE '\s+context deadline exceeded' "$log" &&
    ! grep -vxE "fuzz: elapsed: .*|warning: starting with empty corpus|--- FAIL: ${target} \([0-9.]+s\)|\s+context deadline exceeded|FAIL|exit status [0-9]+|FAIL\s+\S+\s+[0-9.]+s" \
      "$log" | grep -q .
}

log_dir=${FUZZ_LOG_DIR:-}
if [ -z "$log_dir" ]; then
  log_dir=$(mktemp -d)
  trap 'rm -rf "$log_dir"' EXIT
fi

failed=0
packages=$(go list -mod=readonly "${1:-./...}")
for package in $packages; do
  targets=$(go test -mod=readonly "$package" -list '^Fuzz')
  abs=$(go list -mod=readonly -f '{{.Dir}}' "$package")
  dir=".${abs#"$PWD"}"
  while IFS= read -r target; do
    case "$target" in
      Fuzz*)
        echo "Fuzzing $package/$target for ${FUZZTIME:-30s}"
        args=(-mod=readonly "$dir" -run '^$' -fuzz "^${target}$"
          -fuzztime "${FUZZTIME:-30s}" -parallel "${FUZZ_PARALLEL:-2}" -timeout 10m)
        log="$log_dir/$target.log"
        status=0
        if [ -z "${FUZZ_LOG_DIR:-}" ]; then
          go test "${args[@]}" 2>&1 | tee "$log" || status=$?
        else
          go test "${args[@]}" >"$log" 2>&1 || status=$?
        fi
        if [ "$status" -eq 0 ]; then
          [ -z "${FUZZ_LOG_DIR:-}" ] || echo "PASS $target"
        elif fuzztime_race "$log" "$target"; then
          echo "PASS $target (ignored Go fuzz deadline race; no failing input)"
        elif [ -z "${FUZZ_LOG_DIR:-}" ]; then
          failed=1
        else
          echo "FAIL $target (output withheld; see the Linear issue)"
          # Every go-square package feeds celestia-app consensus: priority 1.
          echo "1 $dir $target" >>"$FUZZ_LOG_DIR/failed.txt"
          failed=1
        fi
        ;;
    esac
  done <<< "$targets"
done
exit $failed
