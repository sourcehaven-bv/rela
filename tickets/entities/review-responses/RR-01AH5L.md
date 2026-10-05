---
id: RR-01AH5L
type: review-response
title: Missing strict read is a runtime check only
finding: lateGatedReader detects a reader without ListRelationsStrict at run time.
severity: minor
resolution: Compile-time assertions pin that ScriptReader, UnrestrictedReader and DenyReader implement ListRelationsStrict. The runtime error stays for narrow test doubles and never falls back to the tolerant read.
status: addressed
---
