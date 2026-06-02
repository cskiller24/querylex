---
status: complete
quick_id: 260603-3rg
slug: replace-all-github-com-querylex-querylex
date: 2026-06-02
---

## Summary: Replace github.com/querylex/querylex → github.com/cskiller24/querylex

Replaced all 149 references to `github.com/querylex/querylex` with `github.com/cskiller24/querylex` across 71 files.

### Files changed
- `go.mod` — module path
- ~68 `.go` files — all import paths updated
- `Makefile` — ldflags paths updated
- `.goreleaser.yaml` — ldflags paths + homepage URLs updated
- `README.md` — GitHub release URLs updated

### Verification
- `go build ./...` — passed
- `go vet ./...` — passed
- `go test ./... -short -count=1` — all tests passed (21 packages)
