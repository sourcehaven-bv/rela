---
id: RR-DHOSFM
type: review-response
title: Read errors were swallowed without naming the legitimate case
finding: An unreadable file returned clean with a comment that did not say which other reporting covers it
severity: minor
resolution: The comment now names the legitimate case - fsstore treats git-crypt-encrypted relation files as locked shells - which is what makes skipping defensible rather than merely convenient.
status: addressed
---

`checkRelationFile` returned nil on a read error with a comment saying it is "a
different problem with its own reporting" — without naming that reporting, so a
maintainer could not verify the claim.

The legitimate case is real: fsstore handles git-crypt-encrypted relation files
as locked shells, so an unreadable relation file is expected in an encrypted
repo. The comment now says so, which is what makes skipping defensible rather
than merely convenient.
