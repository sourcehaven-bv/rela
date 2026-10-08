---
id: RR-INX7D4
type: review-response
title: 'Design: operator paths can shadow the sandbox''s own mounts'
finding: spec.ExtraReadOnly is bound after --proc, --dev, --tmpfs /tmp and the writable-dir bind, and SetHostReadOnly accepts any absolute path. Listing / disables read confinement and shadows /proc, /dev, /tmp and the writable dir; listing /tmp or a parent of os.TempDir() makes the writable dir read-only and exposes the host /tmp (other runs' scratch dirs with uploads) to every converter; listing /proc or /dev replaces the namespaced mounts. Docs only say never list /etc; nothing enforces it.
severity: significant
resolution: SetHostReadOnly and WithExtraReadOnly now reject (warn + skip) /, /etc, anything at or under /proc or /dev or containing them, and /tmp or os.TempDir() or any parent of either (unsafeReadPath). Paths are cleaned before the check. TestSetHostReadOnlyRejectsPathsThatUndoTheSandbox covers each case for both entry points, plus near-misses that stay allowed (/devices, /procfs, /tmp/clamd.sock, /etc/paperspecs).
status: addressed
---
