---
id: RR-WRYHU2J
type: review-response
title: Id minting edge cases lack tests
finding: No tests for lower-case prefixes, faced-only sequential types, retired ids or mintShortID failure paths.
severity: minor
resolution: 'Covered in BUG-RLBXTG5: TestCreate_MintsPastMixedCaseIDs (mixed and lower-case ids), TestCreate_ShortIDFailurePaths (count error, lookup error, every candidate taken) and TestCreate_ShortIDMints. Faced-only types and retired ids are pinned by storetest HighestIDSeesFacedEntities and HighestIDCountsRetiredIDs. Lower-case schema prefixes cannot mint at all, a separate defect filed as BUG-4OPP525.'
reason: Existing entity and entitymanager id tests cover the main paths; edge cases tracked with the HighestID contract work.
status: addressed
---
