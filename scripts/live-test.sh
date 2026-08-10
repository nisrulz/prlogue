#!/bin/sh
set -eu

PROJECT_DIR=$(cd "$(dirname "$0")/.." && pwd)
BINARY="$PROJECT_DIR/prlogue"
TESTDIR="$PROJECT_DIR/.temp-test"
CONFDIR="$TESTDIR/conf"
PASS=0
FAIL=0
MOCK_PID=""
MOCK=0

ok() { echo "  ✓ $1"; PASS=$((PASS+1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL+1)); }

cleanup() {
  if [ -n "$MOCK_PID" ]; then
    kill "$MOCK_PID" 2>/dev/null || true
    wait "$MOCK_PID" 2>/dev/null || true
  fi
  rm -rf "$TESTDIR"
}
die() { echo "  ! $1"; cleanup; exit 1; }

# Isolate from the user's real config so the suite never reads or writes it.
PRLOGUE_CONFIG_DIR="$CONFDIR"
export PRLOGUE_CONFIG_DIR

# --- build ---
make -C "$PROJECT_DIR" build || die "build failed"
echo "  • Built prlogue"

rm -rf "$TESTDIR"
mkdir -p "$CONFDIR/prlogue"

# Run prlogue generate in a directory; capture stdout+stderr, never abort the suite.
run() {
  (cd "$1" && "$BINARY" generate ${2:-} 2>&1 || true)
}

# Create a repo with an init commit on the default branch plus a feature branch.
new_repo() {
  repo="$TESTDIR/$1"
  git init -q "$repo"
  git -C "$repo" config user.email "test@test"
  git -C "$repo" config user.name "Test"
  git -C "$repo" commit --allow-empty -m "init" -q
  git -C "$repo" checkout -q -b feature
  echo "$repo"
}

# --- pre-check: select the test provider ---
probe_endpoint() {
  curl -sf "$1/models" >/dev/null 2>&1
}

OLLAMA_URL="http://localhost:11434/v1"
OLLAMA_MODEL="lfm2.5:8b"
LIVE=0
TEST_BASE_URL="$OLLAMA_URL"
if [ "${CI:-}" = "true" ] && [ "${PRLOGUE_LIVE_TEST_PROVIDER:-ollama}" = "mock" ]; then
  (cd "$PROJECT_DIR" && go build -o "$TESTDIR/mock-openai-server" ./scripts/mock-openai-server.go) \
    || die "mock provider build failed"
  "$TESTDIR/mock-openai-server" "$TESTDIR/mock-url" \
    >"$TESTDIR/mock-server.log" 2>&1 &
  MOCK_PID=$!
  i=0
  while [ ! -s "$TESTDIR/mock-url" ] && [ "$i" -lt 50 ]; do
    sleep 0.1
    i=$((i + 1))
  done
  [ -s "$TESTDIR/mock-url" ] || die "mock provider failed to start"
  TEST_BASE_URL=$(tr -d '\n' < "$TESTDIR/mock-url")
  cat > "$CONFDIR/prlogue/config.yaml" <<EOF
provider: openai_compat
model: ci-mock
base_url: ${TEST_BASE_URL}
no_think: true
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
system:
  model_size_gb: 5.2
EOF
  LIVE=1
  MOCK=1
  echo "  ✓ Mock OpenAI-compatible provider started"
else
  echo "  • Probing Ollama at $OLLAMA_URL ..."
  if ! probe_endpoint "$OLLAMA_URL"; then
    die "Ollama is not reachable; start Ollama and pull $OLLAMA_MODEL before running make test-live"
  fi
  LIVE=1
  echo "  ✓ Ollama reachable; live tests run against $OLLAMA_MODEL"
fi

# --- test 1: outside a git repository ---
NONGIT=$(mktemp -d /tmp/prlogue-nongit.XXXXXX)
OUT=$(run "$NONGIT")
rm -rf "$NONGIT"
echo "$OUT" | grep -qi "could not detect default branch\|not a git repository" \
  && ok "detects non-git directory" \
  || fail "should detect non-git directory"

# --- test 2: repo with no changes on the branch ---
repo=$(new_repo nochanges)
OUT=$(run "$repo")
echo "$OUT" | grep -qi "no changes found" \
  && ok "detects no changes" \
  || fail "should detect no changes"

# --- test 3: --staged with nothing staged ---
repo=$(new_repo nostaged)
echo "edited" > "$repo/README.md"
OUT=$(run "$repo" "--staged")
echo "$OUT" | grep -qi "no staged changes found" \
  && ok "detects missing staged changes" \
  || fail "should detect missing staged changes"

# --- test 4: committed changes produce a PR body ---
repo=$(new_repo basic)
cat > "$repo/README.md" <<'EOF'
# My Project
EOF
git -C "$repo" add README.md
git -C "$repo" commit -q -m "feat: add readme"
cat >> "$repo/README.md" <<'EOF'
## Usage
Run make build.
EOF
git -C "$repo" add README.md
git -C "$repo" commit -q -m "docs: add usage section"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "PR Description" \
  && ok "generates a PR body from committed changes" \
  || fail "should generate a PR body"

# --- test 5: staged changes generate a body ---
repo=$(new_repo staged)
echo "new handler" > "$repo/handler.go"
git -C "$repo" add handler.go
OUT=$(run "$repo" "--staged")
echo "$OUT" | grep -qi "PR Description" \
  && ok "generates a PR body from staged changes" \
  || fail "should generate a PR body from staged changes"

# --- test 6: JSON output ---
repo=$(new_repo json)
echo "data" > "$repo/data.txt"
git -C "$repo" add data.txt
git -C "$repo" commit -q -m "feat: add data"
cat > "$repo/.prlogue.yaml" <<'EOF'
output:
  format: json
EOF
OUT=$(run "$repo")
echo "$OUT" | grep -qi '"title"' && echo "$OUT" | grep -qi '"stats"' \
  && ok "emits JSON output" \
  || fail "should emit JSON output"

# --- test 7: binary files do not crash the pipeline ---
repo=$(new_repo binary)
printf '\xff\xd8\xff\xe0\x00\x10\x4a\x46\x49\x46' > "$repo/logo.bin"
git -C "$repo" add logo.bin
git -C "$repo" commit -q -m "chore: add logo"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "PR Description" \
  && ok "handles binary files" \
  || fail "should handle binary files"

# --- test 8: unicode filenames ---
repo=$(new_repo unicode)
echo 'package main' > "$repo/café.go"
git -C "$repo" add café.go
git -C "$repo" commit -q -m "feat: add café module"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "PR Description" \
  && ok "handles unicode filenames" \
  || fail "should handle unicode filenames"

# --- test 9: renamed and deleted files ---
repo=$(new_repo renames)
echo "keep me" > "$repo/keep.txt"
echo "delete me" > "$repo/gone.txt"
git -C "$repo" add -A
git -C "$repo" commit -q -m "feat: add files"
git -C "$repo" mv keep.txt moved.txt
git -C "$repo" rm -q gone.txt
git -C "$repo" commit -q -m "chore: rename and remove files"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "PR Description" \
  && ok "handles renamed and deleted files" \
  || fail "should handle renamed and deleted files"

# --- test 10: special characters in the diff ---
repo=$(new_repo special)
cat > "$repo/special.txt" <<'EOF'
normal line
EOF
git -C "$repo" add special.txt
git -C "$repo" commit -q -m "feat: add file"
cat >> "$repo/special.txt" <<'EOF'
line with $dollar and `backticks`
line with "quotes" and \backslash
EOF
git -C "$repo" add special.txt
git -C "$repo" commit -q -m "chore: add special lines"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "PR Description" \
  && ok "handles special characters in the diff" \
  || fail "should handle special characters in the diff"

# --- test 11: large diff engages chunking ---
repo=$(new_repo large)
i=1
while [ "$i" -le 10 ]; do
  cat > "$repo/file$i.go" <<EOF
package main

func base$i() string {
  return "base $i"
}
EOF
  i=$((i + 1))
done
git -C "$repo" add -A
git -C "$repo" commit -q -m "chore: scaffold files"
i=1
while [ "$i" -le 10 ]; do
  cat >> "$repo/file$i.go" <<EOF

func added$i() string {
  return "added $i"
}
EOF
  i=$((i + 1))
done
git -C "$repo" add -A
git -C "$repo" commit -q -m "feat: extend files"
OUT=$(run "$repo" "-v")
echo "$OUT" | grep -qi "Chunks:" \
  && ok "large diff is chunked" \
  || fail "large diff should be chunked"
echo "$OUT" | grep -qi "PR Description" \
  && ok "large diff completes end to end" \
  || fail "large diff should complete end to end"

# --- test 12: empty repo with no commits ---
repo="$TESTDIR/empty"
git init -q "$repo"
git -C "$repo" config user.email "test@test"
git -C "$repo" config user.name "Test"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "could not detect base branch" \
  && ok "handles an empty repo with no commits" \
  || fail "should handle an empty repo with no commits"

# --- test 13: detached HEAD still generates ---
repo=$(new_repo detached)
echo "content" > "$repo/x.txt"
git -C "$repo" add x.txt
git -C "$repo" commit -q -m "feat: add x"
SHA=$(git -C "$repo" rev-parse HEAD)
git -C "$repo" checkout -q "$SHA"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "PR Description" \
  && ok "generates from a detached HEAD" \
  || fail "should generate from a detached HEAD"

# --- test 14: a nonexistent configured base branch errors cleanly ---
repo=$(new_repo badfrom)
cat > "$repo/.prlogue.yaml" <<'EOF'
git:
  default_branch: nonexistent-branch
EOF
OUT=$(run "$repo")
echo "$OUT" | grep -qi "collect diff" \
  && ok "errors cleanly for a nonexistent configured base branch" \
  || fail "should error cleanly for a nonexistent configured base branch"

# --- test 15: configured base branch rejects revision expressions ---
cat > "$repo/.prlogue.yaml" <<'EOF'
git:
  default_branch: HEAD~10
EOF
OUT=$(run "$repo")
echo "$OUT" | grep -qi "invalid branch" \
  && ok "rejects revision expressions in the configured base branch" \
  || fail "should reject revision expressions in the configured base branch"

# --- test 16: context below the floor is rejected ---
repo=$(new_repo smallctx)
echo "x" > "$repo/x.txt"
git -C "$repo" add x.txt
git -C "$repo" commit -q -m "feat: add x"
cat > "$TESTDIR/smallctx.yaml" <<EOF
provider: openai_compat
model: lfm2.5:8b
base_url: ${TEST_BASE_URL}
context:
  mode: auto
  manual: 100
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
EOF
OUT=$(run "$repo" "--config $TESTDIR/smallctx.yaml")
echo "$OUT" | grep -qi "context lengths must be at least 4096" \
  && ok "rejects a too-small configured context length" \
  || fail "should reject a too-small configured context length"

# --- test 17: non-HTTP configured base_url ---
cat > "$TESTDIR/badurl.yaml" <<'EOF'
provider: openai_compat
model: lfm2.5:8b
base_url: ftp://example.com/v1
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
EOF
OUT=$(run "$repo" "--config $TESTDIR/badurl.yaml")
echo "$OUT" | grep -qi "http or https" \
  && ok "rejects a non-HTTP configured base URL" \
  || fail "should reject a non-HTTP configured base URL"

# --- test 18: unsupported configured output format ---
cat > "$TESTDIR/badformat.yaml" <<EOF
provider: openai_compat
model: lfm2.5:8b
base_url: ${TEST_BASE_URL}
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: xml
EOF
OUT=$(run "$repo" "--config $TESTDIR/badformat.yaml")
echo "$OUT" | grep -qi "output.format must be 'markdown' or 'json'" \
  && ok "rejects an unsupported configured output format" \
  || fail "should reject an unsupported configured output format"

# --- test 19: --output writes the body to a file ---
OUT=$(run "$repo" "--output $TESTDIR/pr.md")
[ -s "$TESTDIR/pr.md" ] && grep -qi "PR Description" "$TESTDIR/pr.md" \
  && ok "writes output to a file" \
  || fail "should write output to a file"

# --- test 20: unknown configured provider ---
cat > "$TESTDIR/bogus.yaml" <<'EOF'
provider: bogus-provider
EOF
OUT=$(run "$repo" "--config $TESTDIR/bogus.yaml")
echo "$OUT" | grep -qi "unsupported provider" \
  && ok "rejects an unknown configured provider" \
  || fail "should reject an unknown configured provider"

# --- test 21: remote openai_compat without PRLOGUE_OPENAI_COMPAT_API_KEY ---
cat > "$TESTDIR/openai.yaml" <<'EOF'
provider: openai_compat
model: gpt-5.6-luna
base_url: https://api.openai.com/v1
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
EOF
OUT=$(cd "$repo" && PRLOGUE_OPENAI_COMPAT_API_KEY= "$BINARY" generate --config "$TESTDIR/openai.yaml" 2>&1 || true)
echo "$OUT" | grep -qi "PRLOGUE_OPENAI_COMPAT_API_KEY is required" \
  && ok "requires PRLOGUE_OPENAI_COMPAT_API_KEY for remote openai_compat" \
  || fail "should require PRLOGUE_OPENAI_COMPAT_API_KEY for remote openai_compat"

# --- test 22: repository config allowlist is honored ---
repo=$(new_repo repoconfig)
echo "data" > "$repo/data.txt"
git -C "$repo" add data.txt
git -C "$repo" commit -q -m "feat: add data"
cat > "$repo/.prlogue.yaml" <<'EOF'
output:
  format: json
EOF
OUT=$(run "$repo")
echo "$OUT" | grep -qi '"title"' \
  && ok "honors the repository config allowlist" \
  || fail "should honor the repository config allowlist"

# --- test 23: repository config rejects disallowed keys ---
repo=$(new_repo badrepoconfig)
cat > "$repo/.prlogue.yaml" <<'EOF'
provider: openai_compat
EOF
OUT=$(run "$repo")
echo "$OUT" | grep -qi "not allowed" \
  && ok "rejects disallowed repository config keys" \
  || fail "should reject disallowed repository config keys"

# --- test 24: corrupt repository config errors cleanly ---
repo=$(new_repo corruptconfig)
printf 'output: [unclosed\n' > "$repo/.prlogue.yaml"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "project config" \
  && ok "errors cleanly on a corrupt repository config" \
  || fail "should error cleanly on a corrupt repository config"

# --- test 25: repository config must be a regular file ---
repo=$(new_repo dirconfig)
mkdir -p "$repo/.prlogue.yaml"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "regular file" \
  && ok "rejects a repository config that is a directory" \
  || fail "should reject a repository config that is a directory"

# --- test 26: explicit --config is honored ---
cat > "$TESTDIR/trusted.yaml" <<EOF
provider: openai_compat
model: lfm2.5:8b
base_url: ${TEST_BASE_URL}
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: json
EOF
repo=$(new_repo explicitcfg)
echo "x" > "$repo/x.txt"
git -C "$repo" add x.txt
git -C "$repo" commit -q -m "feat: add x"
OUT=$(run "$repo" "--config $TESTDIR/trusted.yaml")
echo "$OUT" | grep -qi '"title"' \
  && ok "honors an explicit --config file" \
  || fail "should honor an explicit --config file"

# --- test 27: protected extra_body fields in config are rejected ---
cat > "$TESTDIR/bad-body.yaml" <<EOF
provider: openai_compat
model: lfm2.5:8b
base_url: ${TEST_BASE_URL}
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
extra_body:
  max_tokens: 100000
EOF
OUT=$(run "$repo" "--config $TESTDIR/bad-body.yaml")
echo "$OUT" | grep -qi "protected field" \
  && ok "rejects protected extra_body fields in config" \
  || fail "should reject protected extra_body fields in config"

# --- test 28: untracked-only changes report no changes ---
repo=$(new_repo untracked)
echo "new" > "$repo/untracked.txt"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "no changes found" \
  && ok "reports no changes for untracked-only work" \
  || fail "should report no changes for untracked-only work"

# --- test 29: deletion-only staged diff generates ---
repo=$(new_repo deletion)
printf 'a\nb\nc\n' > "$repo/f.txt"
git -C "$repo" add f.txt
git -C "$repo" commit -q -m "chore: add file"
git -C "$repo" rm -q f.txt
OUT=$(run "$repo" "--staged")
echo "$OUT" | grep -qi "PR Description" \
  && ok "generates from a deletion-only diff" \
  || fail "should generate from a deletion-only diff"

# --- test 30: filenames with spaces ---
repo=$(new_repo spaces)
echo "x" > "$repo/my file.txt"
git -C "$repo" add "my file.txt"
git -C "$repo" commit -q -m "feat: add spaced file"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "PR Description" \
  && ok "handles filenames with spaces" \
  || fail "should handle filenames with spaces"

# --- test 31: mixed change types split into sections ---
repo=$(new_repo mixed)
mkdir -p "$repo/docs"
echo "readme" > "$repo/docs/readme.md"
echo "code" > "$repo/app.go"
git -C "$repo" add -A
git -C "$repo" commit -q -m "feat: add login flow"
OUT=$(run "$repo")
if [ "$LIVE" = "1" ]; then
  echo "$OUT" | grep -qi "PR Description" \
    && ok "generates a PR body for mixed change types" \
    || fail "should generate a PR body for mixed change types"
else
  echo "$OUT" | grep -qi "Features" && echo "$OUT" | grep -qi "Documentation" \
    && ok "splits mixed change types into sections" \
    || fail "should split mixed change types into sections"
fi

# --- test 32: issue refs surface in output ---
repo=$(new_repo issuerefs)
git -C "$repo" checkout -q -b feat/PROJ-123-login
echo "x" > "$repo/x.txt"
git -C "$repo" add x.txt
git -C "$repo" commit -q -m "feat: wire up login"
OUT=$(run "$repo")
if [ "$LIVE" = "1" ]; then
  echo "$OUT" | grep -qi "PR Description" \
    && ok "generates a PR body for the branch" \
    || fail "should generate a PR body for the branch"
else
  echo "$OUT" | grep -qi "PROJ-123" \
    && ok "surfaces issue refs from the branch and commits" \
    || fail "should surface issue refs"
fi

# --- test 33: configured base_url with embedded credentials ---
repo=$(new_repo creds)
echo "x" > "$repo/x.txt"
git -C "$repo" add x.txt
git -C "$repo" commit -q -m "feat: add x"
cat > "$TESTDIR/creds.yaml" <<'EOF'
provider: openai_compat
model: lfm2.5:8b
base_url: http://user:pass@localhost:1234/v1
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
EOF
OUT=$(run "$repo" "--config $TESTDIR/creds.yaml")
echo "$OUT" | grep -qi "must not contain credentials" \
  && ok "rejects credentials in the configured base_url" \
  || fail "should reject credentials in the configured base_url"

# --- test 34: plain HTTP for a non-loopback host ---
cat > "$TESTDIR/httpremote.yaml" <<'EOF'
provider: openai_compat
model: lfm2.5:8b
base_url: http://example.com/v1
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
EOF
OUT=$(run "$repo" "--config $TESTDIR/httpremote.yaml")
echo "$OUT" | grep -qi "https for non-loopback" \
  && ok "rejects plain HTTP for a non-loopback host" \
  || fail "should reject plain HTTP for a non-loopback host"

# --- test 35: --output to an unwritable path ---
OUT=$(run "$repo" "--output /nonexistent-prlogue-dir/x.md")
echo "$OUT" | grep -qi "write output" \
  && ok "errors cleanly when --output is unwritable" \
  || fail "should error cleanly when --output is unwritable"

# --- test 36: oversized repository config ---
repo=$(new_repo hugeconfig)
head -c 70000 /dev/zero | tr '\0' '#' > "$repo/.prlogue.yaml"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "exceeds" \
  && ok "rejects an oversized repository config" \
  || fail "should reject an oversized repository config"

# --- test 37: oversized diff is bounded ---
repo=$(new_repo hugediff)
head -c 12582912 /dev/zero | tr '\0' 'x' > "$repo/big.txt"
git -C "$repo" add big.txt
git -C "$repo" commit -q -m "chore: add huge file"
OUT=$(run "$repo")
echo "$OUT" | grep -qi "exceeds" \
  && ok "bounds an oversized diff" \
  || fail "should bound an oversized diff"

# --- test 38: custom prompt in config still generates ---
cat > "$TESTDIR/customprompt.yaml" <<EOF
provider: openai_compat
model: lfm2.5:8b
base_url: ${TEST_BASE_URL}
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
prompt: Write a PR description for a Go codebase.
EOF
repo=$(new_repo customprompt)
echo "x" > "$repo/x.txt"
git -C "$repo" add x.txt
git -C "$repo" commit -q -m "feat: add x"
OUT=$(run "$repo" "--config $TESTDIR/customprompt.yaml")
[ -n "$OUT" ] \
  && ok "accepts a custom prompt in config" \
  || fail "should accept a custom prompt in config"

# --- test 39: oversized configured prompt is rejected ---
cat > "$TESTDIR/bigprompt.yaml" <<EOF
provider: openai_compat
model: lfm2.5:8b
base_url: ${TEST_BASE_URL}
context:
  mode: auto
  manual: 131072
  max_auto: 1000000
  min_auto: 4096
chunking:
  strategy: two-tier
  file_summary_threshold: 200
  hunk_split_threshold: 500
output:
  format: markdown
prompt: $(head -c 70000 /dev/zero | tr '\0' 'p')
EOF
OUT=$(run "$repo" "--config $TESTDIR/bigprompt.yaml")
echo "$OUT" | grep -qi "prompt must not exceed" \
  && ok "rejects an oversized configured prompt" \
  || fail "should reject an oversized configured prompt"

# --- test 41: doctor verifies a healthy setup (mock provider only) ---
if [ "$MOCK" = "1" ]; then
  repo=$(new_repo doctor)
  echo "x" > "$repo/x.txt"
  git -C "$repo" add x.txt
  git -C "$repo" commit -q -m "feat: add x"
  OUT=$(cd "$repo" && "$BINARY" doctor 2>&1 || true)
  echo "$OUT" | grep -qi "All checks passed." \
    && ok "doctor reports a healthy setup" \
    || fail "doctor should report a healthy setup"
fi

# --- report ---
echo "  ─────────────────────────────"
echo "  Results: $PASS passed, $FAIL failed"

cleanup

[ "$FAIL" -eq 0 ]
