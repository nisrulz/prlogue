#!/bin/sh
# Streams test results as they run, then prints a pass/fail summary and the
# list of failed tests. Exits with the test command's exit code.
set -eu

tmpdir="$(mktemp -d)"
fifo="$tmpdir/log.fifo"
log="$tmpdir/log.txt"
counts="$tmpdir/counts.txt"
mkfifo "$fifo"
trap 'rm -rf "$tmpdir"' EXIT

"$@" >"$fifo" 2>&1 &
pid=$!

# Readable display name: for a subtest "Parent/leaf" show "leaf" with
# underscores back as spaces; for a plain Go test strip the "Test" prefix and
# split camelCase / underscores into words ("TestRunDoctor_AllChecksPass" ->
# "Run Doctor All Checks Pass").
leaf_name() {
  case "$1" in
    */*) printf '%s' "${1##*/}" | tr '_' ' ' ;;
    *)
      printf '%s' "$1" | sed -E \
        -e 's/^Test//' \
        -e 's/_/ /g' \
        -e 's/([a-z0-9])([A-Z])/\1 \2/g' \
        -e 's/([A-Z])([A-Z][a-z])/\1 \2/g'
      ;;
  esac
}

# tee saves the full log; the shell loop reads line by line as go test emits
# them and prints a compact ✓ / ✗ marker per test. Package verdict lines
# (PASS/ok/FAIL) are replaced by a separator between groups. The results are
# piped through test-renderer, which animates a spinner between lines and owns
# the terminal cursor so the spinner never collides with a result line. Each
# printf is a separate write, so output streams as tests finish. The second
# tee records what was displayed so the report can count and list it exactly.
msg="Running tests"
case " $* " in
  *" ./e2e/"*) msg="Running end-to-end tests" ;;
esac
sep_line='  ─────────────────────────────'
tee "$log" <"$fifo" | {
  parents=""
  sep=""
  while IFS= read -r line; do
    case "$line" in
      "=== RUN "*)
        run=${line#*=== RUN }
        case "$run" in
          */*)
            parent=$(printf '%s' "${run%%/*}" | tr -d ' ')
            if [ -n "$parent" ]; then
              parents="$parents $parent"
            fi
            ;;
        esac
        ;;
      *"--- PASS: "*)
        if [ -n "$sep" ]; then
          printf '%s\n' "$sep_line"
          sep=""
        fi
        name=${line#*--- PASS: }
        name=${name% (*}
        case "$name" in
          */*) printf '  ✓ %s\n' "$(leaf_name "$name")" ;;
          *)
            case " $parents " in
              *" $name "*) ;;
              *) printf '  ✓ %s\n' "$(leaf_name "$name")" ;;
            esac
            ;;
        esac
        ;;
      *"--- FAIL: "*)
        if [ -n "$sep" ]; then
          printf '%s\n' "$sep_line"
          sep=""
        fi
        name=${line#*--- FAIL: }
        name=${name% (*}
        case "$name" in
          */*) printf '  ✗ %s\n' "$(leaf_name "$name")" ;;
          *)
            case " $parents " in
              *" $name "*) ;;
              *) printf '  ✗ %s\n' "$(leaf_name "$name")" ;;
            esac
            ;;
        esac
        ;;
      *"--- SKIP: "*)
        if [ -n "$sep" ]; then
          printf '%s\n' "$sep_line"
          sep=""
        fi
        name=${line#*--- SKIP: }
        name=${name% (*}
        case "$name" in
          */*) printf '  - %s (skipped)\n' "$(leaf_name "$name")" ;;
          *) printf '  - %s (skipped)\n' "$(leaf_name "$name")" ;;
        esac
        ;;
      PASS) ;;
      ok*) sep=1 ;;
      FAIL*)
        case "$line" in
          FAIL) ;;
          *) sep=1 ;;
        esac
        ;;
    esac
  done
} | tee "$counts" | go run ./scripts/test-renderer "$msg"
rc=$?
wait "$pid" || rc=$?

passed=$(grep -c '^  ✓ ' "$counts" || true)
failed=$(grep -c '^  ✗ ' "$counts" || true)
skipped=$(grep -c '^  - ' "$counts" || true)

echo ""
echo "  ─────────────────────────────"
summary="  Results: $passed passed, $failed failed"
if [ "$skipped" -gt 0 ]; then
  summary="$summary, $skipped skipped"
fi
echo "$summary"
if [ "$failed" -gt 0 ]; then
  echo ""
  echo "  Failed Tests:"
  echo ""
  grep '^  ✗ ' "$counts"
  echo ""
fi
echo "  ─────────────────────────────"

exit "$rc"
