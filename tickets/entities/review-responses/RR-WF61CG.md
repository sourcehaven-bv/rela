---
id: RR-WF61CG
type: review-response
title: Own delete versus moderator remove on one's own message
finding: '§0 and AC 7: the service treats DELETE with a reason from a moderator as a moderator removal, even on their own message (kind moderator, reason kept), and DELETE without a reason as the author''s delete (kind author, no reason). A moderator looking at their own message needs both choices, labelled differently. Otherwise the UI picks one silently. Fix: show "Delete" (own) and "Remove…" (moderator, reason dialog) separately, and test both bodies.'
severity: minor
resolution: Delete and Remove are separate buttons, each driven by its own _actions entry, and both request bodies are tested (R21).
status: addressed
---
