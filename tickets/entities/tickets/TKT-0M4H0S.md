---
id: TKT-0M4H0S
type: ticket
title: 'Access PR 1: golden wire-parity corpus, test ACL helpers, verifiedIdentity rename'
kind: refactor
priority: medium
effort: l
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

## Description

Access PR 1 of TKT-WFS778 (internal/access). Groundwork that the later PRs are
checked against; no production behaviour change.

1. **Golden wire-parity corpus.** An in-process test in internal/dataentry records, for each principal and endpoint, the status, the normalized response body and the store-read count (storetest.Counting) on a fixture project with an acl.yaml (row gating, `visible:` fields with `when:`, faces, relations to hidden entities, attachments, history). Results live in testdata; `-update` regenerates them. Each later access PR must reproduce them byte for byte.
2. **Test codemod.** Replace the ~366 direct `app.acl = ...` and `app.fieldResolver = ...` assignments in dataentry tests with `setTestACL` / `setTestResolver` helpers, so PR 6 can remove the App fields by changing only the helpers.
3. **Rename** `acl.Policy.RejectEffective(jwtWired)` to `verifiedIdentity` (acl/policy.go, dataentry/router.go).

Parent: TKT-WFS778. Plan: .ignored/TKT-WFS778-design.md, PR sequence item 1.
