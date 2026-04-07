---
id: RR-LOFJ
type: review-response
title: Path validation reinvents script.Engine's hardened pattern
finding: Plan uses HasPrefix after Clean, vulnerable to symlinks, case-insensitive FS, and TOCTOU. Reuse os.OpenRoot pattern from script/executor.go.
severity: critical
resolution: Reuse script.Engine's existing os.OpenRoot-based loadScript. New ExecuteAction method uses the same hardened path validation.
status: addressed
---
