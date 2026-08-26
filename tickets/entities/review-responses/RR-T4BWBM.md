---
id: RR-T4BWBM
type: review-response
title: Cascade read deps still passed a nil field redactor
severity: critical
status: addressed
finding: >-
  The automation-cascade read path (the static lua.ReadDeps built in assemble)
  still passed nil as its field redactor, four lines below where fieldRedactor
  was already in scope — a third instance of the exact defect this ticket
  exists to remove. Unlike the two sites that were fixed, it carried no KNOWN
  LIMITATION note, so nothing warned a reader it was unenforced.
resolution: >-
  Passed fieldRedactor at both cascade sites. Pinned by
  TestCascadeReadDeps_RedactsHiddenField, an end-to-end test driving a real
  PatchEntity, automation trigger and Lua read. Mutation-verified: without the
  fix the hidden salary lands as "99000" on an unrestricted field.
---

The cascade path is identity-bearing by the same definition that justified
fixing the other two: it runs on the acting user's ctx and reads their view
(DEC-O59WM4, RR-XC0URX), and a Lua action can send what it reads onward exactly
as a scheduled job can.

**Root cause of the miss:** I fixed the two sites that were *documented* rather
than searching for the *pattern* — a `nil` passed at a redactor parameter. The
documentation was the map, and the map was incomplete. A KNOWN LIMITATION note
records where someone already looked, not where the problem is.
