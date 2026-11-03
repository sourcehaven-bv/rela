---
id: RR-MJPHMM
type: review-response
title: TestButtons_DeciderAgrees should also check shadow-hidden buttons and outputs
finding: The test checks only that a listed button moves to the entry's target. Also assert that for a principal from whom a button is hidden by a shadowing entry, posting the preset moves along the shadowing entry's target, and that the entry's outputs ran (the mark names that entry). This pins both directions of the load-time shadow set against the decider.
severity: minor
resolution: TestButtons_DeciderAgrees now gives each entry an output mark and asserts that a shadow-hidden preset moves along the shadowing entry and writes its mark.
status: addressed
---
