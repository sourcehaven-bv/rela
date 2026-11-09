---
id: RR-DLXB809
type: review-response
title: A local delete is undone after the source briefly hides the row
finding: guard forgot ledger rows in state dropped when the source stopped listing them, so a later listing recreated the deleted entity.
severity: minor
resolution: guard keeps dropped rows. TestRun_LocalDeleteIsNotRecreated now hides and relists the row; it fails without the fix.
status: addressed
---
