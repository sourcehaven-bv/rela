---
id: RR-JILP
type: review-response
title: Form view bypasses Edit-button guard — direct URL or keyboard shortcut routes to empty form
finding: 'EntityDetail.vue:236-243 hides Edit button + early-returns editEntity when isInaccessible. But /form/<id>/<entityId> is still routable. Browser bookmark, copy-paste URL, or pressing E during the brief window when entity.value is null then loads as inaccessible — bypasses the guard. Form opens with empty Properties (because the entity''s Properties IS empty), user fills required fields, hits Save. Server''s 422 saves us — EXCEPT when finding #1 fires. Fix: detect entity.inaccessible in FormView.vue and render the same banner as EntityDetail with form widgets disabled. Don''t rely solely on the server.'
severity: significant
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
