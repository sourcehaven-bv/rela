---
id: RR-ESFLWV
type: review-response
title: Rendered bytes held before an upload slot is taken
finding: The render buffer was allocated before uploads.TryAcquire, so md renders were unbounded in concurrency.
severity: minor
resolution: The slot is taken after the write check and before the render, and released when the call returns.
status: addressed
---
