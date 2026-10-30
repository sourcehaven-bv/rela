---
id: RR-6C9VSGE
type: review-response
title: Legacy types / empty regex / fuzzy untested
finding: No test for =~ on legacy status or priority types. No empty regex case. Nothing pins that fuzzy is still rejected on enum.
severity: minor
resolution: Added TestMatchLegacyTypeRegex and table cases for an empty regex and for fuzzy being rejected.
status: addressed
---
