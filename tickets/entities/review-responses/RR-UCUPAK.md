---
id: RR-UCUPAK
type: review-response
title: Retired store's index hides records written after a save
finding: The replaced generation's fsstore saves its index on Close stamped with the folder times at that moment. Records the new generation wrote meanwhile (a whole new type folder in the reproduction) were missing from it, and a restart trusted the index and did not list them. Found while adding an acceptance test for criterion 5.
severity: significant
resolution: retire drops the persisted index after closing the old store (appbuild.DropStoreIndex via app.FSFactory.DropStoreIndex), so the next start scans the files. TestConfigure_NewTypeSurvivesRestart fails without the fix.
status: addressed
---
