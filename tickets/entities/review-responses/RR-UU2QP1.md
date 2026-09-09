---
id: RR-UU2QP1
type: review-response
title: os.IsNotExist vs errors.Is - two idioms for one check in the same package
finding: removeAttachmentDir uses the wrapping-safe form while the relation loop uses the PathError-shaped one
severity: minor
resolution: Switched both checks to errors.Is(err; os.ErrNotExist) for consistency with removeAttachmentDir and to survive a future wrapping FS. Also added the slog.Warn for the already-gone case - it is the only signal that index and disk had drifted before the call.
status: addressed
---

The restructured if/else is behaviourally equivalent for the already-gone case
and `s.echoes.Forget` still runs on every path (it sits after the if/else,
outside both branches, as before) — verified, no regression.

But `os.IsNotExist` only matches `*PathError`-shaped errors, not a bare
`fs.ErrNotExist` sentinel, while the adjacent `removeAttachmentDir` uses
`errors.Is(err, os.ErrNotExist)` — the modern, wrapping-safe form. Two idioms
for the same check in one package; prefer `errors.Is` so it survives any future
FS wrapper that wraps its errors.

Separately: a relation file that was already gone is skipped and reported
nowhere — correct for audit (this call did not delete it), but it means silent
index/disk drift has no signal at all. A `slog.Warn` costs nothing.
