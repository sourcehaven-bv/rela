---
id: RR-HVCNYA
type: review-response
title: Face-delete fixme did not check edges
finding: The title claims the draft's edges go and the published face stays, but only face existence was asserted; an over-eager cascade would pass.
severity: critical
resolution: After the delete the test opens POL-1@published and asserts CTL-3 (its content edge) and CTL-2 (shared identity edge) remain and CTL-1 is gone.
status: addressed
---

The title claims the draft's edges go and the published face stays, but only
face existence was asserted; an over-eager cascade would pass.
