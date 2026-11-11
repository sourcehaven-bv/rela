---
id: RR-O8GTNTS
type: review-response
title: Visible holder reads on every hint without debounce
finding: A visible holder called read() for each hint, one full inbox read per round trip under a burst.
severity: minor
resolution: A visible holder now waits VISIBLE_DEBOUNCE_MS (250 ms) so a burst costs one read. Test sends two hints and expects one read.
status: addressed
---
