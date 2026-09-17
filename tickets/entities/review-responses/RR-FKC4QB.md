---
id: RR-FKC4QB
type: review-response
title: Broken environment exited 0 with a reassuring message instead of failing
finding: 'scripts/check-tagged-tests.sh: `mapfile -t ALL_TAGS < <(discover_tags)` runs discovery in a process substitution, whose exit status is discarded, so `pipefail` was inert. A missing root, an unreadable tree or a missing go binary all fell through to "nothing to compile" and exit 0 — the false-negative shape arriving by the error path rather than the parse path. Demonstrated with RELA_ROOT=/nonexistent: find errored to stderr and the guard still exited 0.'
severity: significant
resolution: 'Added explicit pre-flight checks that exit 2: root must be a directory, the discovery program must exist, and go must be on PATH. Discovery failure is now detected via command substitution and reported with its output. Cases "missing root exits 2" and "unknown argument exits 2" cover it.'
status: addressed
---
