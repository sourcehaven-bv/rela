---
id: RR-DA86NZ
type: review-response
title: rename onto a deleted id is untested
finding: The storetest ordering never exercises the d.Vseq >= r.Hi branch of CurrentLifecycle.
severity: significant
resolution: storetest RenameOntoAnIDDeletedAfterTheTag and DeleteAfterRenameDropsTheTag.
status: addressed
---
