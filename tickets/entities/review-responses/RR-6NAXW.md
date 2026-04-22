---
id: RR-6NAXW
type: review-response
title: E2E test asserts on prototype ticket count — fragile to fixture changes
finding: internal/dataentry/e2e_test.go. Test checks 'Tickets (' appears in rendered HTML; breaks subtly if the prototype's belongs-to relations for backend are removed. Either seed tickets explicitly in the test's project copy or comment that the assertion is robust to zero tickets (it is — the header still appears).
severity: nit
resolution: Expanded the AC comment in TestE2E_LuaDocumentRenders to explicitly document that the 'Tickets (' assertion survives fixture changes (the header appears independent of ticket count).
status: addressed
---

From post-impl cranky review. Add a clarifying comment rather than change
assertion semantics.
