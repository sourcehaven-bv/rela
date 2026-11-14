---
id: TKT-R98U9GZ
type: ticket
title: Next-action rereads a whole source when the suggestion on screen is past the window
kind: enhancement
priority: low
status: backlog
---

## Description

Since TKT-CKLZMWB a next-action query source reads the first candidates in
windows. When the suggestion on screen (`current_entity`) is not among them,
`queryCandidates` reads the whole source again so the engine can keep it on
screen. The SPA sends the last suggestion on every navigation, also after it
stopped matching, so this full read still happens.

Check the one id instead: run the same gated, pushed query narrowed to that id
(`store.MatchingIDs` or a GraphQuery with an id predicate), apply the Go filter
pass, and append the row when it matches. That bounds the case to one or two
statements.
