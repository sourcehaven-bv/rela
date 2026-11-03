---
id: RR-SEAVNH
type: review-response
title: Processes can disagree on the flag between reloads
finding: The flag is decided at append time from each process's own compiled config. During a reload window the web server, an MCP stdio process and the CLI may hold different realtime.yaml versions, so two posts of the same type can be stored with different flags. This follows from the settled "applies to new messages" rule. One sentence in the GUIDE-realtime "applies from the next post" paragraph would stop it from being reported as a bug.
severity: nit
resolution: ADDRESSED GUIDE-realtime "applies from the next post" paragraph gains a sentence that processes may disagree on the flag during a reload; listed as an edge case.
status: addressed
---
