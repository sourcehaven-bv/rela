---
id: RR-NEDPI23
type: review-response
title: Conformance-suite concurrency subtests lost -race before merge
finding: go test -run matches each name level separately, so the top-level name filter skipped the concurrency subtests in storetest (CAS, VersionTags), commentstest, leasetest, pilestest, jobstest and visibilitytest.
severity: significant
resolution: scripts/ci-race-gate.sh also runs those subtests by full path; verified locally with -v that they run.
status: addressed
---
