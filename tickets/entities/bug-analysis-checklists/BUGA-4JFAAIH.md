---
id: BUGA-4JFAAIH
type: bug-analysis-checklist
title: 'Analysis: Raw not-found errors, unmarked step fields, entity wording and requests after an ended session'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (Go handler test and the fake API for the 422; vitest for the step form, the confirm and the client)
- [x] Minimal reproduction steps documented (open `/entity/feature/FEAT-99999`, `/list/x`, `/form/x`; submit an empty step form; answer one request with 401 and count the next)
- [x] Environment/conditions noted (saas-dev pool behind the edge; any server for the not-found cases)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (uniform 404 for a missing view entry; NotFoundState shared by the not-found page, entity page, list and form; field messages from the blocked step; label on the select; request guard in the API client; stream stops on a refusal; isQuietFailure at the log sites)
- [x] Regression test planned (Go view tests, vitest per surface, e2e not-found addresses)
- [x] Related areas checked for similar issues (the world-absent path answers a missing entry the same way; the side panel reuses EntityDetail; the provisioning poll and the session probe use fetch, not the client)
