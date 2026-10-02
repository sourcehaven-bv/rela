---
id: RR-UDRKK8
type: review-response
title: ID@face passes existence checks in whole-entity tools
finding: delete, rename, trace and find_path accepted ID@face at the existence check and then used the fused string as an id.
severity: significant
resolution: wholeEntityRef refuses an ID@face address in those tools with a message naming the bare id. TestWholeEntityRef.
status: addressed
---
