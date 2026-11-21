---
id: RR-ZHIW9X2
type: review-response
title: Tests inject the boot language instead of reading the meta
finding: The tests set the project language through resetLanguageState, so the real path where locale.ts reads the rela-language meta was not run, and no test covered loaded going false again.
severity: minor
resolution: The tests write the meta tags and import locale.ts and the composable fresh, as on a page load, and a reload case was added. Mutating the guard or the latch fails a test. App.vue itself is not mounted, because its setup needs the router, the stores and the shell; the composable is the whole of the logic.
status: addressed
---
