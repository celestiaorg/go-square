#!/usr/bin/env bash
set -euo pipefail

# Discover targets so new tests are automatically included in nightly runs.
# Listing/compilation errors must fail the job, not silently omit a package.
packages=$(go list -mod=readonly "${1:-./...}")
for package in $packages; do
  targets=$(go test -mod=readonly "$package" -list '^Fuzz')
  while IFS= read -r target; do
    case "$target" in
      Fuzz*)
        echo "Fuzzing $package/$target for ${FUZZTIME:-30s}"
        go test -mod=readonly "$package" -run '^$' -fuzz "^${target}$" \
          -fuzztime "${FUZZTIME:-30s}" -parallel "${FUZZ_PARALLEL:-2}" -timeout 10m
        ;;
    esac
  done <<< "$targets"
done
