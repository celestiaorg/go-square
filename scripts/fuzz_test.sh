#!/usr/bin/env bash
# Tests for scripts/fuzz.sh. A stub `go` on PATH replays canned fuzz output, so
# no real fuzzing runs. Run with: bash scripts/fuzz_test.sh
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/fuzz.sh"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/bin" "$work/repo/pkg"
cat >"$work/bin/go" <<'EOF'
#!/usr/bin/env bash
# Stub: list one package with one target; the fuzz run prints $FAKE_OUTPUT and
# exits with $FAKE_EXIT.
case "$*" in
  *"-f {{.Dir}}"*) echo "$PWD/pkg" ;;
  "list "*) echo "example.com/pkg" ;;
  *"-list ^Fuzz"*) printf 'FuzzTarget\nok\texample.com/pkg\t0.01s\n' ;;
  *"-fuzz "*) printf '%s\n' "$FAKE_OUTPUT"; exit "$FAKE_EXIT" ;;
  *) echo "unexpected go $*" >&2; exit 2 ;;
esac
EOF
chmod +x "$work/bin/go"

deadline_only='fuzz: elapsed: 5m0s, execs: 9269832 (0/sec), new interesting: 1 (total: 393)
--- FAIL: FuzzTarget (300.11s)
    context deadline exceeded
FAIL
exit status 1
FAIL	example.com/pkg	300.127s'

crash='fuzz: elapsed: 5m0s, execs: 100 (0/sec), new interesting: 1 (total: 393)
--- FAIL: FuzzTarget (300.11s)
    --- FAIL: FuzzTarget (0.00s)
        context deadline exceeded
    Failing input written to testdata/fuzz/FuzzTarget/abc
    To re-run:
    go test -run=FuzzTarget/abc
FAIL'

# The race during crash minimization: the crasher is saved but only the
# deadline is reported, so this must fail.
minimizing='fuzz: elapsed: 4m59s, execs: 100 (0/sec), new interesting: 1 (total: 393)
fuzz: minimizing 57-byte failing input file
--- FAIL: FuzzTarget (300.11s)
    context deadline exceeded
FAIL'

extra_error='--- FAIL: FuzzTarget (300.11s)
    context deadline exceeded
    open testdata/fuzz/FuzzTarget/abc: permission denied
FAIL'

hung='--- FAIL: FuzzTarget (12.00s)
    fuzzing process hung or terminated unexpectedly: exit status 2
FAIL'

failures=0
# expect <name> <want exit> <output> <fuzz exit> [log dir]
expect() {
  local name=$1 want=$2 output=$3 fuzz_exit=$4 log_dir=${5:-}
  local got=0
  (cd "$work/repo" && PATH="$work/bin:$PATH" FAKE_OUTPUT="$output" \
    FAKE_EXIT="$fuzz_exit" FUZZ_LOG_DIR="$log_dir" bash "$script" ./pkg) \
    >"$work/out" 2>&1 || got=$?
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: exit $got, want $want"
    sed 's/^/    /' "$work/out"
    failures=$((failures + 1))
  else
    echo "ok   $name"
  fi
}

expect "pass" 0 "ok" 0
expect "fuzztime race is not a failure" 0 "$deadline_only" 1
expect "fuzztime race is not a failure (log dir)" 0 "$deadline_only" 1 "$work/logs1"
expect "crasher fails" 1 "$crash" 1
expect "crasher fails (log dir)" 1 "$crash" 1 "$work/logs2"
expect "race while minimizing a crasher fails" 1 "$minimizing" 1
expect "race with another error fails" 1 "$extra_error" 1
expect "hung worker fails" 1 "$hung" 1
expect "hung worker fails (log dir)" 1 "$hung" 1 "$work/logs3"

if [ -s "$work/logs1/failed.txt" ]; then
  echo "FAIL fuzztime race was listed in failed.txt"
  failures=$((failures + 1))
fi
if ! grep -q FuzzTarget "$work/logs2/failed.txt"; then
  echo "FAIL crasher missing from failed.txt"
  failures=$((failures + 1))
fi

[ "$failures" -eq 0 ]
