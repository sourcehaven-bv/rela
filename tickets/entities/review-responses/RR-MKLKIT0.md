---
id: RR-MKLKIT0
type: review-response
title: Row hides newer messages from the decider
finding: Only the waiting message is shown; later messages (a co-owner objecting, a runner correction) are invisible and nothing says they exist.
severity: significant
resolution: Entries carry last_seq; the row shows N newer messages with a button that opens the drawer when last_seq > cause + 1 (AC 11a). Whether to also hide one-click buttons then is listed as an open choice.
status: addressed
---
