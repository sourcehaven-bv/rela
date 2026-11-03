---
id: RR-3UK6Y0
type: review-response
title: 'Regression test was vacuous: it passed on the pre-fix code'
finding: TestDetectFromNodeWithMetamodel_DoesNotLeakMetamodelAcrossRuns seeded run 1 with an empty mockMetamodel. With no property types the cleanup migration detects nothing in either run, so the loop over run 2's detections never executed and the test passed against the pre-fix commit 8d77eb1d5.
severity: significant
resolution: Run 1 now uses a metamodel that types ticket.title as string, so the text widget is redundant and run 1 must detect it (asserted with t.Fatal). A leaked metamodel makes run 2 detect it too. Verified the test now fails on 8d77eb1d5 and passes with the fix.
status: addressed
---

Found by running the new test against the pre-fix tree. An assertion over an
empty result set proves nothing; the test now asserts the precondition that
makes a leak observable.
