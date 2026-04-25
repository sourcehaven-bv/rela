---
id: RR-GYIPB
type: review-response
title: AGENTS.md doesn't fold the new rule into the enforcement bullet list
finding: The 'Page Object Pattern (enforced)' section at lines 10-15 lists the four currently-enforced primitives (locator, getBy*, waitForTimeout, request.fetch). The new section at line 43 documents a fifth banned primitive but is not folded into that bullet list. A reader scanning for 'what's banned' will read the bullets, miss the new section, and conclude `api.rawRequest` is free.
severity: minor
resolution: Added `api.rawRequest(...)` and `api['rawRequest'](...)` as a fifth bullet in the 'Page Object Pattern (enforced)' list (e2e/tests/AGENTS.md:15), with a pointer down to the dedicated 'API-only assertions belong in Go' section. The dedicated section was also rewritten to reflect the simplified rule (no hook exemption) and to point at internal/dataentry/ as the right home for HTTP-shape tests.
status: addressed
---
