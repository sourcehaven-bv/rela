---
id: RR-6SKLU7
type: review-response
title: CLI wiring test bypassed the wiring it claimed to cover
finding: The CLI test set svc.Comments by hand, so newCLIBundles, the migrate data/gc Comments fields and the server GC wiring were all unexercised.
severity: significant
resolution: Added appbuild.Collaborators.Comments and appbuildtest.WithComments; CLI tests for adopt-face, migrate data (drop_entities) and migrate gc now go through newCLIBundles. Server GC deps extracted to gcDeps with TestGCDeps_CommentService. Removing each wiring line fails at least one test (verified).
status: addressed
---
