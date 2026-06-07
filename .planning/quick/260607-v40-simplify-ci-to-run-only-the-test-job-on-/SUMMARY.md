---
status: complete
quick_id: 260607-v40
date: 2026-06-07
description: Simplify CI to run only the test job on ubuntu-latest
---

## Summary

Simplified `.github/workflows/ci.yml` to keep only the `test` job running on ubuntu-latest.

### Changes

- Removed `lint` job (golangci-lint)
- Removed `build-check` job with 3-OS matrix
- Simplified `test` job: removed 3-OS matrix, single `runs-on: ubuntu-latest`

### Verification

- `go build ./...` — passes
- `go test ./... -short -count=1` — passes
