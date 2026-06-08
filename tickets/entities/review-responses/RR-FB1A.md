---
id: RR-FB1A
type: review-response
title: 'C1: lastSeenServer is private — AC 6 cannot be implemented as written'
finding: |
  PLAN-IHC7B AC 6 reads `useAutoSave.lastSeenServer[prop]` but that's a closure-local in useAutoSave.ts (L155); it's not on the returned object. Verdict-flip semantics require a different source for the "value to revert to".
severity: critical
status: addressed
resolution: |
  Adopt option (c) from the review: use `viewData.entry.properties[prop]` as the authoritative server state on verdict flip — the host already owns it via the response of `loadView()`. AC 6 rewritten in PLAN: "on verdict-flip, SectionEditForm calls `useAutoSave.revertField(prop)` (which already exists and reverts via the apply* callback) and surfaces the toast." No useAutoSave API change needed.
---
