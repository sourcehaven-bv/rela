---
id: RR-L7NGRP
type: review-response
title: Family-of-id header read duplicated across six packages
finding: cli storedFamily, aclmap target, sync localType, dataentry loadStoredFamilies, docs facesOf and ScheduledForEachPrincipal each read an id's faces from headers, with small differences.
severity: minor
resolution: PR 6 (#1735) added store.FamilyHeaders and store.Family in internal/store/family.go. The single-id family reads in cli/address.go and aclmap and sync push and docs and scheduler for_each now call it. dataentry loadStoredFamilies stays separate because it batches many ids into one header read.
reason: A shared store helper is part of the store API flip in PR 6; adding it here would change the store API this PR is told to keep.
status: addressed
---
