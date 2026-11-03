---
id: RR-AG0UJH
type: review-response
title: Plural form at zero and label fallbacks changed behaviour
finding: DynamicForm clear/attach counts now pick the plural form for 0; ChannelsSection/OpenChannel treat an empty label as missing; exports CONFLICT_MESSAGE, DEFAULT_BUCKET_LABELS and NONE_TITLE became functions.
severity: nit
reason: Count 0 cannot occur at those call sites; treating an empty label as missing is the better behaviour; the renamed exports have no other importers and a stale importer fails typecheck.
status: wont-fix
---
