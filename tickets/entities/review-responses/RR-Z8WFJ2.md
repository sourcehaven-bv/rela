---
id: RR-Z8WFJ2
type: review-response
title: Containment test was rejection-shaped, contrary to dataentry/CLAUDE.md
finding: 'TestMintFileToken_RejectsOutsideProject asserted only that no token was minted. internal/dataentry/CLAUDE.md warns specifically against this: ''Assert containment (no file outside custom/ is ever served), never the request errors — a rejection-shaped test passes against a leaky implementation.'' The test never asserted that the bytes of a file outside the project root are unreachable. Also flagged: the frontend helper runWithSSE(body) ignored its parameter (all three call sites passed ''''), which read as though it set up an empty stream.'
severity: minor
resolution: Added TestMintFileToken_OutsideBytesAreUnreachable, which writes a known secret outside the root, asserts neither the secret nor the path appears in the SSE payload, and — if a token were somehow minted — downloads it and asserts the secret is not served. The property is now pinned directly rather than inferred from a rejection. Fixed runWithSSE to call stubSSE(body); mutation-tested by reverting it, which fails 2 of 7 tests.
status: addressed
---
