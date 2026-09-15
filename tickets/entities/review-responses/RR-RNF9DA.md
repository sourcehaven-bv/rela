---
id: RR-RNF9DA
type: review-response
title: find | while read subshell and unguarded filenames
finding: 'scripts/check-tagged-tests.sh used `find ... | while IFS= read -r f`, whose loop body runs in a subshell: any state assigned there would be discarded at `done`. Harmless as written (the loop only wrote to stdout) but primed to silently discard a future error flag or counter in a script whose entire purpose is not passing silently. `find -print` with `read -r` also breaks on filenames containing newlines.'
severity: significant
resolution: 'Moot: the shell discovery loop was deleted entirely when discovery moved to Go. filepath.WalkDir handles arbitrary filenames and returns errors up the stack, which the caller now checks and exits 2 on.'
status: addressed
---
