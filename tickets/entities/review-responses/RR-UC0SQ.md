---
id: RR-UC0SQ
type: review-response
title: govulncheck output may contain attacker-influenced text; consider safer body interpolation
finding: Heredoc `<<EOF` (not `<<'EOF'`) runs shell expansion on the log body. OSV descriptions upstream could theoretically contain `$(...)` or backticks that'd get evaluated. Narrow surface (GitHub issue body only), but worth hardening — use `printf '%s\n' ...` to build BODY.
severity: nit
resolution: security.yml heredoc now uses `<<'BODY_EOF'` (quoted — no shell expansion) with `__RUN_URL__`/`__LOG__` placeholders substituted via bash parameter expansion. Verified with a test script that `$(...)` and backticks in the log body are preserved literally.
status: addressed
---
