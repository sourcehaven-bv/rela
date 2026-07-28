---
id: RR-YI4LGQ
type: review-response
title: 'listOverrideRenderer godoc asserted a false invariant: the override widens the list surface to include entity bodies'
finding: The doc comment claimed the override "renders exactly the set the on-screen view showed". Rows reach the script via EntityToTable, which includes Content, and visibility.Redact does not redact Content (the body-redaction TODO in internal/visibility/policyreader.go). The built-in column table never renders bodies, so choosing an override widens the field surface for the same request. Not a new ACL hole — row-gating and property redaction are intact, and the entity export path already exposes bodies — but the comment asserted an invariant that is not true, which is the kind of comment that suppresses the search when body redaction is later implemented.
severity: significant
resolution: Kept the behavior (a report override legitimately wants bodies, and blocking them here would diverge from the entity export path) and corrected the claim. listOverrideRenderer now states that the override sees the same ROWS with the same property redaction but NOT only what the table showed, and both it and the lua.ListRows seam godoc point at the body-redaction TODO so whoever implements it finds this path.
status: addressed
---
