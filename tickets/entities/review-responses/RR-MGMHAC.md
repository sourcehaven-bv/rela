---
id: RR-MGMHAC
type: review-response
title: 'display: nested in a form side_panel builds a tree the wire drops'
finding: 'buildSections has two callers: the _views handler and executeSidePanel. A side panel reuses ViewSection, so display: nested there reaches the new arm and builds a tree -- but v1.SidePanelSection (apiwire/v1/responses.go) carries only Fields and Entities, so the tree is silently dropped on the way out. Worse, side panels are not validated for display mode at all: validateSidePanelSpans is the only validator that descends into form.SidePanel (its own doc comment says so), so validateNestedSection never runs there. Net effect: a section with a heading, isEmpty false, and nothing in it -- exactly the silent-wrongness validateNestedSection exists to prevent.'
severity: significant
resolution: 'Refused at config load in validateSidePanelSpans: a side_panel section with display: nested is now an error naming the form and section index. Rejecting rather than plumbing Tree through SidePanelSection, since nothing has asked for a tree in a narrow panel and the wire type would need Columns/Rows too. Note the same hole pre-exists for display: table and group_by in side panels; not widened here, but worth its own ticket.'
status: addressed
---
