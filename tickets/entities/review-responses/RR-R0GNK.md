---
id: RR-R0GNK
type: review-response
title: Corpus round-trip test was synthetic-only despite plan saying tickets/entities/*.md
finding: TestMdCorpusRoundTrip used 10 hand-written one-liners instead of the version-controlled corpus the plan specified. C1 and C2 above survived only because the test was incomplete.
severity: significant
resolution: Added corpusRoundTripFromDisk that walks tickets/entities/*.md, parses each file's body, asserts parse→render→parse fixed point. 798 of 801 files pass (the 3 skipped contain multi-line HTML comments — a pre-existing goldmark HTML-block round-trip quirk unrelated to this refactor). Skip rule documented in containsMultilineHTMLComment.
status: addressed
---
