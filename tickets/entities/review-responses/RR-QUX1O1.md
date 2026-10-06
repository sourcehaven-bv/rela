---
id: RR-QUX1O1
type: review-response
title: e2e 'editor not open' assertion can pass before Milkdown mounts
finding: toHaveCount(0) on .ProseMirror passes immediately while the editor builds asynchronously.
severity: minor
resolution: Fixed. Spec first asserts the read view (swapped out synchronously on edit) is still visible.
status: addressed
---
