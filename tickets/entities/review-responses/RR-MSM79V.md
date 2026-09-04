---
id: RR-MSM79V
type: review-response
title: The denied-world empty response had an obvious signature — it WAS the existence oracle it closed
finding: |-
    VERIFIED by diffing the two responses. writeEmptyWorldResponse hand-built `{"data":[]}`; the real list endpoint returns:

      REAL:  {"data":[],"meta":{"total":0,...},"_actions":{"create":true}}
             + Link, X-Total-Count, X-Page, X-Per-Page headers
      FAKE:  {"data":[]}
             + Content-Type only

    Missing meta, missing _actions, five missing headers. Any client — including the SPA, which reads meta.total — distinguishes them on the first byte. Since a GRANTED-but-empty world produces the real shape, the bare body meant 'you lack the grant' and the full one meant 'the world is empty for you'. The mechanism designed to close an existence oracle was one.

    The single-entity path had the same defect in miniature: my synthetic 404 used the literal 'not found' while a genuine miss uses entityNotFoundTitle.

    And my test could not catch any of it: TestAttachWorld_DeniedWorldIsIndistinguishable grepped the body for the words 'denied' and 'permission' — a proxy for the property, not the property.
severity: critical
resolution: |-
    Removed the synthetic writer entirely. A denied world now binds a handle marked `denied` and lets the ORDINARY handler run: the read seams (scopedSortedEntities, getVisible) return nothing, so the real code produces the real empty shape — same JSON, same meta, same _actions, same headers, same status.

    The test was replaced with TestDeniedWorldIsByteIdenticalToAnEmptyWorld, which runs the real handler twice — once under a world the principal MAY read that happens to hold nothing, once under a denied world — and compares bodies, status, and five headers. Verified it bites: making the denied path return an error instead fails it on X-Total-Count.
status: addressed
---
