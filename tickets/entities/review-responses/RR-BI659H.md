---
id: RR-BI659H
type: review-response
title: unsafeReadPath is lexical; symlinks bypass it
finding: On macOS /tmp is refused but /private/tmp passes, and the darwin backend EvalSymlinks-resolves it into a subpath socket allow covering launchd/ssh-agent listener sockets. On Linux any symlink pointing at a refused path passes. Check both the literal and the resolved path.
severity: minor
resolution: unsafeReadPath checks the path as written and its symlink-resolved form (resolving through the longest existing ancestor for paths that do not exist yet), against protected directories that are themselves also resolved (so /private/tmp and /private/etc on macOS are covered). Comparison is case-insensitive on macOS. TestSetHostReadOnlyChecksSymlinkTargets covers links to /, /dev and /etc and a future path under a link.
status: addressed
---
