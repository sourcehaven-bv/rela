---
id: RR-Z0AHAY
type: review-response
title: ID-encoding guarantee miscited; wantsNewTab misnamed; row tab-stops 0->N
finding: 'Three items: the plan''s edge cases required testing entity IDs containing #/?//spaces/unicode, which internal/entity/id.go:134 makes impossible (ValidateID pins IDs to ^[A-Za-z0-9][A-Za-z0-9_-]*$), so the security reasoning cited encoding when the real guarantee is the grammar; wantsNewTab was misnamed since it also covers defaultPrevented, which means the opposite of ''open a tab''; and row tab stops go from 0 to N per page, a real interaction change with the existing j/k keyboard model.'
severity: minor
resolution: The helper is named shouldDeferToBrowser, with a comment explaining why (defaultPrevented means nothing should happen, not open a tab) and that callers use it only to skip their own push. The plan now cites entity/id.go's ValidateID as the path-safety guarantee instead of encoding, and names cellLink as the genuinely unconstrained input — which is what the new hostile-link tests cover. The 0-to-N tab-stop change is accepted deliberately (links should be reachable) with one anchor per row, not per cell; the row itself stays non-focusable as before.
status: addressed
---

**Finding (S2/M1/M4, minor cluster).** Three smaller corrections.

**S2 — wrong guarantee cited.** The plan's edge cases require testing IDs
containing `#`, `?`, `/`, spaces, unicode. Verified impossible:
`internal/entity/id.go:134` pins every ID to `^[A-Za-z0-9][A-Za-z0-9_-]*$`, and
`ValidateID` is documented as the single validity rule across the codebase
(TKT-IZGF7T). `encodeURIComponent` is therefore a no-op and the plan's security
reasoning rests on encoding when the real guarantee is the grammar. **Cite
`entity/id.go` instead**, so a future relaxation of the grammar trips the right
wire. The genuinely untested input is `cellLink`, a server-supplied string with
no such grammar — that is where a malformed-href test belongs.

**M1 — `wantsNewTab` name lies.** Including `e.defaultPrevented` is
behaviourally right (all call sites are `if (...) return`) but semantically
inverted: a defaultPrevented event means *nothing* should happen, not "open a
tab". Rename to `shouldDeferToBrowser` / `shouldSkipRouterPush`, or split the
check out. Otherwise a later `if (wantsNewTab(e)) trackNewTabOpened()` logs
phantom opens.

**M4 — tab-stop count changes from 0 to N.** Rows currently have *zero* tab
stops (keyboard access is the j/k `useListKeyboard` model). A focusable title
anchor per row adds one per row — on a 50-row list that is 50 new tab stops.
Probably correct (links should be reachable), but it is a real interaction
change with the existing keyboard model and needs an explicit decision, not a
risk row implying nothing changed.

**M2 — alt-click note.** On macOS, Option-click on an anchor *downloads* the
target, so a row link would silently drop an HTML file in ~/Downloads. Staying
consistent with `useDocumentClicks.ts:33` (which includes `altKey`) is the right
call; just record the consequence.
