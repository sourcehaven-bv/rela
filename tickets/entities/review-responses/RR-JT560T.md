---
id: RR-JT560T
type: review-response
title: Four backends in one ticket is disproportionate for stage 1
finding: The plan's own risk section concedes 'four backends is the bulk of the work, and three are new storage code', and the ticket is already effort:l with a UI half. Three of the four (postgres, sqlite, file) are genuinely new persistence code, each needing migrations or a serialisation format, plus conformance runs — and the postgres one is DB-gated in CI so it reads as uncovered by default. The conformance suite makes adding a backend cheap LATER, which is the argument for landing the interface plus the default file backend and memory (enough to ship the feature and prove the seam), then adding pg/sqlite as a follow-up ticket. This is a sequencing suggestion, not an objection to the design — the interface must still be designed for all four up front so the later ones need no interface change.
severity: minor
resolution: Accepted. This ticket now ships two backends — filecomments (default) and memcomments (tests) — with pgcomments/sqlitecomments moved to a follow-up ticket. The interface is still designed for all four so the later ones need no interface change, and commentstest.RunAll is what makes adding them cheap.
status: addressed
---
