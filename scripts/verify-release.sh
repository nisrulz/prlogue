#!/bin/sh
# Fails when a built binary is not an optimized release binary.
# A release binary must be built with -trimpath and a stripped symbol table.
# Usage: scripts/verify-release.sh [binary ...]
# With no arguments, it checks every binary in the goreleaser dist directory.
set -eu

verify_one() {
	binary=$1
	problems=""

	go version -m "$binary" 2>/dev/null | grep -q -- '-trimpath=true' ||
		problems="build without -trimpath; it leaks the build machine paths"

	if [ "$(go tool nm "$binary" 2>/dev/null | grep -c ' [Tt] ')" -gt 0 ]; then
		problems="symbol table is not stripped; build with -ldflags '-s -w'"
	fi

	if [ -n "$problems" ]; then
		echo "  ✗ $binary: $problems"
		return 1
	fi
	echo "  ✓ $binary is an optimized release binary"
}

if [ "$#" -gt 0 ]; then
	binaries=$*
else
	binaries=$(find dist -type f \( -name 'prlogue' -o -name 'prlogue.exe' \) 2>/dev/null | sort)
fi

if [ -z "$binaries" ]; then
	echo "✗ No built binary found to verify"
	exit 1
fi

echo "Verifying release build settings..."
status=0
for binary in $binaries; do
	verify_one "$binary" || status=1
done

[ "$status" -eq 0 ] || exit 1
echo "✓ All binaries are optimized release builds"