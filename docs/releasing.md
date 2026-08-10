# Releasing

PRlogue uses GoReleaser to build binaries for Linux, macOS, and Windows.

## Before tagging

You need write access to the repository and GoReleaser if you want to test the package locally.

Run the same checks as CI:

```bash
make audit
go mod verify
go mod tidy -diff
```

To build a local snapshot:

```bash
make snapshot
```

GoReleaser writes snapshot artifacts to `dist/`.

## Create a release

Tag the commit and push the tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Tags matching `v*` start the release workflow. It uses Go 1.25 and runs formatting, module, race, vet, and vulnerability checks before GoReleaser starts.

The workflow publishes:

- Linux binaries for amd64 and arm64
- macOS binaries for amd64 and arm64
- Windows binaries for amd64 and arm64
- `tar.gz` archives, Windows zip files, and `checksums.txt`

The workflow actions are pinned to commit hashes. Update the hash and its version comment together when upgrading an action.

## Versioning

Use Semantic Versioning:

- Patch releases fix bugs.
- Minor releases add backward-compatible behavior.
- Major releases contain breaking changes.
