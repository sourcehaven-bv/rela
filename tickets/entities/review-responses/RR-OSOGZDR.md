---
id: RR-OSOGZDR
type: review-response
title: No rollback test for retired ids
finding: RunRetiredIDTests covers only a committed Tx. Nothing asserts that a rolled-back rename leaves no retired_ids row.
severity: minor
resolution: storetest RolledBackRenameRetiresNothing covers a rename rolled back inside Tx.
status: addressed
---
