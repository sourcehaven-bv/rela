---
id: RR-GLB5OO
type: review-response
title: Every read error is reported as a symlink problem
finding: Any non-not-found Load error said 'not a regular file, or a link that leaves lib/' and the cause was discarded.
severity: minor
resolution: Cancellation/timeout and permission errors get their own messages; other errors are logged with slog and the message points at the server log. Pinned by TestRequire_UnreadableModule.
status: addressed
---
