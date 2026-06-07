---
quick_id: 260607-v40
slug: simplify-ci-to-run-only-the-test-job-on-
date: 2026-06-07
status: complete
---

# Quick Task 260607-v40: Simplify CI to run only the test job on ubuntu-latest

## Task

Simplify `.github/workflows/ci.yml` to keep only the `test` job on ubuntu-latest, removing the 3-OS matrix, lint job, and build-check job.

### 1. Edit ci.yml

- **File:** `.github/workflows/ci.yml`
- **Action:** Remove `lint` and `build-check` jobs, keep only `test` job with `runs-on: ubuntu-latest` (no matrix)
- **Verify:** `go build ./...`, `go test ./... -short -count=1` pass
