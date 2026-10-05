---
id: RR-QZCY6M
type: review-response
title: A face-gate error is silently answered as not-found
finding: ReadableFaces turned a FaceSetGate or FaceGate error into NoFaces with no log, so a backend fault at gate step 3 became an unlogged 404 while the contract said only row-gate errors are returned.
severity: significant
resolution: ReadableFaces now returns (FaceSet, error); the resolver returns the error like a row-gate error (it happens before any load, so it discloses nothing). The Resolver doc names both gate steps. New gate-order case "readable faces error" pins it with zero store reads.
status: addressed
---
