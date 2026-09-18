---
id: RR-YDYLZ5
type: review-response
title: Section button resolver has no ACL gate; the plan's central mitigation cites a call that does not exist
finding: resolveSectionButtonsWithTraverse (internal/dataentry/sections.go:436-528) contains zero calls to computeCollectionActions or acl. It derives add targets from createFormForType(et) != "" alone, which is a pure config lookup with no principal involved. The affordance means "a form exists", not "you may create one". The plan's central security mitigation and its Existing Solutions table both described the permission check as existing prior art needing only to be called from a new path; it must in fact be written. AC1 and AC10 rested on it.
severity: critical
resolution: 'Plan corrected: Existing Solutions table now states the check is not called at all and must be wired in; a new subsection documents the absence, the viewsHandler/affordanceService wiring requirement, the LinkInfo split, and the _sidepanel behaviour change for the release note.'
status: addressed
---

## Finding

The plan's principal security mitigation — "derive from
`computeCollectionActions`, the same call the list handler uses" — describes a
call the resolver does not make.

Verified: `resolveSectionButtonsWithTraverse`
(`internal/dataentry/sections.go:436-528`) contains **zero** calls to
`computeCollectionActions` or anything in `acl`. It derives add targets from
`createFormForType(et) != ""` alone, and `createFormForType`
(`views_handler.go:773-795`) is a pure config lookup over `s.Cfg.Forms` with no
principal involved. The current affordance therefore means "a form exists", not
"you may create one".

The plan's "Existing Solutions" table compounded this by listing the permission
check as existing prior art merely needing to be "called from the view path".

Three consequences the plan did not budget for:

1. AC1 and AC10 rest on a call that must be **written**, not rewired.
2. `viewsHandler` must reach `affordanceService`; if it does not already hold one,
that is a constructor change subject to the nil-required-fields rule.
3. `_sidepanel` gains a gate it lacks today — a behaviour change on a shipped
surface, with its own tests to update.

## Resolution

Plan corrected. The "Existing Solutions" table now states the check is not
called at all and must be wired in. A new subsection under Relation threading
documents the absence, the three consequences, and names the `_sidepanel`
behaviour change as a fix that belongs in the release note.
