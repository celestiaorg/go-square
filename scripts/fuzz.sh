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
        if [ -z "${FUZZ_LOG_DIR:-}" ]; then
          go test "${args[@]}" || failed=1
        elif go test "${args[@]}" >"$FUZZ_LOG_DIR/$target.log" 2>&1; then
          echo "PASS $target"
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
