---
id: RR-AFFWVWQ
type: review-response
title: Singleton semantics underspecified
finding: NULL faces defeat the partial unique index; constraint errors are ambiguous; a stored finished run blocks.
severity: minor
resolution: subject_face NOT NULL DEFAULT ''; explicit checks under the subject lock give ErrExists or ErrSubjectBusy; contract says a stored singleton blocks until deleted and many-to-one keeps old runs non-singleton.
status: addressed
---
