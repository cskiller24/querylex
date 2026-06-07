---
status: complete
quick_id: 260606-jkr
date: 2026-06-06
description: change root command default behavior to show --help output instead of custom message
---

## Summary

Changed `RootCmd.Run` in `internal/rootcmd/root.go:73` from printing a custom message to calling `cmd.Help()`, so running `querylex` without arguments now displays the full help output (same as `querylex --help`).

### Changes

- `internal/rootcmd/root.go` — replaced `fmt.Println("Querylex: use 'querylex add-db'...")` with `cmd.Help()`
