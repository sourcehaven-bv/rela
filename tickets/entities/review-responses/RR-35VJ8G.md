---
id: RR-35VJ8G
type: review-response
title: Prefer Intl-only over @date-fns/tz unless a prototype proves the need
finding: 'Plan adds @date-fns/tz for arbitrary-zone conversion. But the only two operations needed - render a UTC instant in zone X, and convert a wall-clock-in-zone-X to UTC - are both doable with Intl.DateTimeFormat({timeZone}) + a ~30-line offset helper, no bundled tzdata (Intl uses the OS db), no new dep. Dependency additions are one-way doors here. Recommendation: prototype Intl-only first (it also cleanly handles non-integer offsets like +05:30 that kill naive math); only pull @date-fns/tz if the Intl approach proves insufficient, and if so note transitive footprint + CVE check. Removes the supply-chain/bundle concern entirely if Intl suffices.'
severity: significant
reason: 'User elected to add @date-fns/tz upfront rather than prototype Intl-only (''any reason not to just add the tz lib?''). @date-fns/tz relies on Intl for the tz database (no bundled tzdata), is small, and is the primitive VueDatePicker uses. Implementation task added: verify no CVEs (npm audit) and note transitive footprint when adding. Intl-only remains a documented fallback if the lib disappoints.'
status: wont-fix
---
