---
id: RR-2S0ZP8
type: review-response
title: 'History read: type-confusion oracle — gate trusts URL {type}, never checks entity''s real type (cross-type history leak)'
finding: 'cranky-code-reviewer C1: authorizeHistoryRead (history_handler.go:98-101) does a.reader.getEntity(id) (ungated, id-only), and on found calls gateReadOrNotFound(typeName, id) with typeName from the URL — but NEVER checks entity.Type == typeName. The real GET handler does (api_v1.go:901). PermitsRead resolves the read-query BY TYPE, and the store keys history by ID only, so a principal denied ''ticket'' but allowed ''note'' requests /_history/note/TKT-SECRET → the note read-query (AllowAll) passes → full ticket history (every snapshot, every property) leaks. Confirmed real. serveHistoryVersion compounds it: redacts under the SNAPSHOT''s type verdicts, not the denied type''s.'
severity: critical
resolution: 'Fixed: authorizeHistoryRead now checks live.Type == typeName (indistinguishable 404 on mismatch); serveHistoryVersion and restoreHistoryVersion check snap.Type == typeName. So the URL type can no longer borrow another type''s read verdict. Regression test TestHandleV1History_CrossTypeIs404 seeds a live ticket, asserts /_history/note/<ticket-id> returns 404 with no content leak, and the same-type request returns 200.'
status: addressed
---
