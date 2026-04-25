---
id: RR-1G4EK
type: review-response
title: 'No composable test for id_type: sequential'
finding: 'useEntityIDControls has no unit test for id_type: sequential combined with multi-prefix. The composable handles it correctly via the !isManual branch, but the case isn''t pinned by a test.'
severity: minor
reason: showPrefixPicker is gated by `mode === 'create' && !isManual.value && prefixOptions.value.length > 1`. The 'short, multi-prefix' tests in the same file already cover the !isManual + multi-prefix path, since the composable's logic does not branch on short vs sequential. Adding a row that only differs by id_type string would be testing string equality rather than behavior. The prototype-only sequential+multi-prefix question is tracked in RR-R8W1A.
status: wont-fix
---
