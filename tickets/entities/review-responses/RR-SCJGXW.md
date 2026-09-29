---
id: RR-SCJGXW
type: review-response
title: Dry-run conflict check was case-sensitive
finding: planRename compared newID exactly while both stores refuse a rename onto an id equal under case folding, so a dry run of POL-1 -> pol-3 with POL-3 present succeeded while the real rename failed.
severity: significant
resolution: planRename now uses idTakenByOther, a content-free header scan that folds case and ignores the entity's own family, so abc -> ABC still plans. Pinned by TestFamilyRename_DryRun (POL-3, pol-3 and Pol-1 targets).
status: addressed
---
