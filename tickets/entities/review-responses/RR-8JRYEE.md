---
id: RR-8JRYEE
type: review-response
title: The summary line claimed a content disagreement for files whose content was never read
finding: An unparseable filename has no content mismatch - it was counted under a message saying it did
severity: minor
resolution: Summary reworded to 'with filename/content problems'; each reason now renders with its own consequence (skipped entirely / empty type / indexed as the wrong triple).
status: addressed
---

Every finding rolled into `"Found %d relation file(s) whose name and content
disagree"`, including `filename is not FROM--TYPE--TO` — where the file was
never opened. An operator greps for a file "whose content disagrees", opens it,
finds content that agrees perfectly, and stops trusting the check.

Reworded to "with filename/content problems", and each reason now renders with
its own explanation of the consequence (skipped entirely / empty type / indexed
as the wrong triple).
