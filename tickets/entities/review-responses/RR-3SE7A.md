---
id: RR-3SE7A
type: review-response
title: Add --strict flag to rela create/update/set
finding: 'Plan rejected exit-code-2-on-warnings. Fine. But no escape hatch on create/update commands. CI scripts relying on rela update exit code lose validation gate. Plan''s implicit answer: ''use rela validate separately'' — forces two-step CI dance. make''s -W, gcc''s -Werror are precedent. Recommendation: add --strict (or --warnings-as-errors) to rela create/update/set. Default: exit 0 on warnings. Set: warnings → exit 1 with same stderr output. ~2 lines per command. If rejected, document rationale beyond ''use a different command''. From design-review F14.'
severity: minor
resolution: '--strict flag added to rela create / update / set (Layer 4 spec: ~3 lines per command). Default exit 0; --strict elevates warnings to exit 1 with same stderr output. AC17 covers the behavior. Documented on each command''s --help text.'
status: addressed
---
