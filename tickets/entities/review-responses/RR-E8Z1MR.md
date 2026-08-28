---
id: RR-E8Z1MR
type: review-response
title: /_config leaks gated document names, permissions, and script/command strings
finding: 'handleV1Config served the whole dataentryconfig.DocumentConfig map (api_v1.go:1341) plus raw Navigation (1340) to every principal. That defeats the deny path''s uniform 404: rather than probing /_documents/<guess>, a caller reads the document names straight out of _config — along with the `permission:` value naming the grant to seek, the `script:` path, and the `command:` shell string (the last a pre-existing leak). The non-enumerability property was tested at one endpoint, documented in docs/data-entry.md, and recorded in internal/dataentry/CLAUDE.md, while being false at another endpoint in the same file.'
severity: critical
resolution: |-
    REVERSED after user correction. The finding assumed document names are confidential; they are not. data-entry.yaml is an operator-authored file in the repo — routinely a public one, as in any open-source app — so its keys, script paths and permission values are already disclosed. The defect was my documented claim that names were non-enumerable, not the endpoint that contradicted it.

    Everything built to satisfy the claim has been removed: the v1.Document wire type, visibleDocuments/visibleNavigation filtering, and TestConfig_HidesGatedDocuments. /_config again serves dataentryconfig.DocumentConfig verbatim. The permission: deny is now a 403 naming the document and permission instead of a disguised 404, which is actionable for the operator.

    Separately and worse: the sidebar filtering I added contradicted an existing recorded decision — docs/acl-security.md § 'Sidebar menu structure is principal-independent' states the menu is served identically to every principal because 'the metamodel is not a secret' and names per-principal hiding as a tightening deliberately not done. Neither planning nor the review found it. Now pinned by TestSidebarAndConfig_PrincipalIndependent and recorded as a rule in the root CLAUDE.md ('The configuration is not a secret; the data is') so the correction generalizes beyond this ticket.
reason: |-
    The finding's premise is false, so the fix was reverted rather than kept. It assumed configured document names are confidential. They are not: data-entry.yaml is an operator-authored file that lives in the repo — routinely a public one, as in any open-source app — so its keys, script paths and permission values are already disclosed to anyone who can read the source. Filtering them out of the API concealed nothing while asserting a secrecy property that never held.

    The real defect was mine, one layer up: I documented a non-enumerability claim in docs/data-entry.md and internal/dataentry/CLAUDE.md that the codebase never supported. The reviewer correctly spotted the contradiction but resolved it in the wrong direction — the claim should have been deleted, not defended with machinery.

    Worse, the sidebar filtering I wrote contradicted an existing recorded decision: docs/acl-security.md § 'Sidebar menu structure is principal-independent' states the menu is served identically to every principal because 'the metamodel is not a secret (it's served by /api/v1/_schema)', and explicitly names per-principal menu hiding as a tightening deliberately not done, 'for no confidentiality gain'. Neither my planning nor the code review surfaced it.

    What replaced it is strictly better for operators: the permission deny is a 403 naming the document and the required permission, instead of a 404 pretending the document does not exist. The actual confidentiality boundary — the ACL-gated reader bounding what a document's Lua can read — was never touched by any of this.

    Guarded going forward by TestSidebarAndConfig_PrincipalIndependent, by a new rule in the root CLAUDE.md ('The configuration is not a secret; the data is') that generalizes to every config surface, and by a pointer to the acl-security.md decision in internal/dataentry/CLAUDE.md.
status: wont-fix
---
