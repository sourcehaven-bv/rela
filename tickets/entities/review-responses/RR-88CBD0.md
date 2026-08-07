---
id: RR-88CBD0
type: review-response
title: Guard itself has no tests; /tmp reuse and missing-assets-dir message are minor sharp edges
finding: (a) No test proves the guard FAILS when it must — an untested guard on the release path is how TKT-O03TB rotted to nothing; verification currently lives only in a terminal history. (b) /tmp/spa-verify is reused without cleanup; a tar failure aborts under GH's bash -e today, so no false pass, but relying on set -e semantics to prevent a stale-artifact pass is the same implicit reasoning that caused this bug. (c) When assets/ is entirely absent the message says 'produced an index.html with no bundle' — right verdict, wrong diagnosis. (d) Only linux_amd64 is checked; defensible since one embed source feeds all platforms, but it should say so.
severity: minor
resolution: (a) Added scripts/check-embedded-spa-test.sh — 17 cases covering every negative shape (missing dir, empty dir, no assets dir, zero-byte asset, missing referenced asset, zero-byte index.html, embed-error-strings-only binary, stale-build binary, arity/usage). It runs in the release job before the guard, so the guard is verified on every release rather than trusted. (b) Each archive now extracts into its own mktemp -d and removes it after, so no reuse and no reliance on set -e to avoid a stale-artifact pass. (c) Missing assets/ dir now has its own distinct message. (d) Added a comment stating why one linux/amd64 archive per pair suffices (all platforms built from the same embed source in the same job).
status: addressed
---
