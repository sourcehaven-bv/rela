---
id: RR-N8UUS1
type: review-response
title: Incoming relation lists are not guarded
finding: The relations token covers outgoing edges only and mergeRelations passes incoming keys through. An incoming-list edit replaces the list and deletes a concurrent edge without a 412.
severity: significant
reason: 'Pre-existing gap outside this ticket: incoming lists are loaded by the relation picker, not the entity GET, so guarding them needs a new token and picker plumbing. Tracked in TKT-E9WXPU; the API docs state that incoming lists are not guarded. Deferred by the user.'
status: deferred
---
