---
id: RR-AWE0WX
type: review-response
title: AC 4 and the Seq == 1 path cannot be exercised with the existing fixture
finding: The "create instant" subtest cannot work. `instant` has cardinality one and no `create:`, so Service.Create refuses it with 409 not_creatable (service.go:735-738). Its only message type, `note`, moves working to done (busy_rows_test.go:59-68), so the first post creates the channel and leaves the working state in the same append, and the card is never busy. The fixture declares no `lifecycle:`, so close and delete are refused as not offered (lifecycle.go:115-118). The one case that justifies the `Seq == 1` rule, a first post that creates an implicit channel in a working initial state without moving it, therefore has no integration test. AC 1's unit table proves only the mark kind. Extend busyRowsRealtime and name it in the test plan. Add a non-moving message type to `instant` (or a new type) so that a first post turns the card busy through Seq == 1. Add a cardinality-many type with `create:` and a working initial state, for "create". Declare lifecycle close and delete on a session type, for "close" and "delete".
severity: significant
resolution: 'Adopted: busyRowsRealtime gains a job type (implicit, starts working, non-moving log message, declared close and delete) and a task type (cardinality many, create:, starts working), and AC 4 names subtests for the first post, create, close and delete on them.'
status: addressed
---
