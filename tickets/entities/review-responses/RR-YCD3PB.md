---
id: RR-YCD3PB
type: review-response
title: Regex/dash-check indirection + unused Prefix field
finding: validIDPrefixBase permits dashes (constrained two lines later) which reads indirect; InvalidIDPrefixError.Prefix is carried but not rendered (Reason embeds it).
severity: nit
resolution: Comment added explaining the split (targeted error messages). Prefix field kept as a structured field for programmatic inspection, mirroring the package's error style.
status: addressed
---
