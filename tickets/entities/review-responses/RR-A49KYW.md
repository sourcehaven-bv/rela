---
id: RR-A49KYW
type: review-response
title: S1 hardcoded field anchor + minors (cp portability, toast-only gate, mintID collision)
finding: 'S1: firstFieldAnchor hardcoded ''title'' as the WaitVisible fallback — an entity type whose form has no #field-title would time out. Minors: N1 copyDirIfExists shells out to cp -r with context.Background() (non-portable, non-cancellable, follows symlinks); N2 the old gate only recognized toast-error not toast-warning; N3 mintID could collide an explicit id with a later auto-minted one.'
severity: minor
resolution: 'S1 DISSOLVED by the C2 fix — the renderability gate now polls the form-state data-testid, not a specific field, so no field anchor is needed for the gate at all (per-annotation anchors still fail loud on their own targets). N2 moot (the gate no longer uses toasts). N1 left as documented: the source is a trusted local project dir and cp -r matches the e2e test; os.CopyFS is a noted future portability improvement. N3 self-correcting (raw-store CreateEntity errors loudly on a duplicate id) — acceptable for fixtures.'
status: addressed
---
