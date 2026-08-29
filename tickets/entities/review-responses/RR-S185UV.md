---
id: RR-S185UV
type: review-response
title: rela-desktop bricks on re-selecting the already-open project
finding: LoadProject calls appbuild.New (opening the new store) at main.go:152 and closes the previous services at :205 -- deliberately outside the mutex. That order only worked because no prior backend held an exclusive resource. sqlitestore takes the flock BEFORE opening the DB, so opening project X while X is already open hits its own lock and fails permanently, with an error naming the SAME process as the culprit. Reachable from openProjectFromMenu and the recent-projects menu, where re-selecting the open project is an ordinary click. rela-desktop is also absent from .goreleaser.yaml and from every CI isolation assertion, so nothing would have caught it.
severity: critical
resolution: 'Reproduced first (a probe opening the same path twice failed with ''held by pid <self>''), then fixed by tearing down before opening. Extracted into releaseLoadedProject so the ordering requirement is documented at the site where someone would otherwise reverse it, and so LoadProject stays under the funlen limit. The consequence is stated in the doc comment: a load that then fails leaves NO project open rather than the previous one, which is the honest outcome. Added CI assertions that rela-desktop links neither pgx nor the sqlite driver.'
status: addressed
---
