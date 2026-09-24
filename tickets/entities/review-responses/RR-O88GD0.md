---
id: RR-O88GD0
type: review-response
title: sqlite CLI reconcile would drop every index for packaged projects
finding: Reading data-entry.yaml with fs.ReadFile treats a packaged project as wanting no indexes; all-or-nothing rule broken.
severity: significant
resolution: Spec loading moves to a shared appbuild helper on the config.Loader seam; the sqlite CLI and recipe both use it; an unreadable or invalid config skips reconcile.
status: addressed
---
