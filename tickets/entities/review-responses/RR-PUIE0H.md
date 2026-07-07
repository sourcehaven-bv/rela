---
id: RR-PUIE0H
type: review-response
title: -12px top margin hand-tuned against header margin-bottom
finding: '.list-info--top { margin-top: -12px } is tuned against .list-header { margin-bottom: 24px } 40 lines away, with no test to catch drift. Acceptable for a single call site but note the coupling in a comment.'
severity: minor
resolution: Expanded the CSS comment on .list-info--top to explicitly document the coupling to .list-header's margin-bottom and a keep-in-sync note.
status: addressed
---
