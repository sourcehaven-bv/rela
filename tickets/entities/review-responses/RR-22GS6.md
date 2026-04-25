---
id: RR-22GS6
type: review-response
title: Consider labelling button 'Edit <type>' to disambiguate multi-entity docs
finding: 'For docs that are anchored to one entity but walk many (e.g. a release-notes doc anchored to a release that summarises tickets), a generic ''Edit'' button is ambiguous — users will assume it edits one of the listed items. Since we''re already gating on entity_type, labelling the button ''Edit {{ docConfig.entity_type }}'' (e.g. ''Edit release'') is one template line and removes a class of misclicks. Decide explicitly: keep generic ''Edit'' or use the typed label.'
severity: nit
resolution: 'Obviated by the redesign: the button label is now author-controlled (edit.label in data-entry.yaml). Authors disambiguate however they want (''Edit ticket'', ''Edit release'', ''Open in form'', etc.) without code changes.'
status: addressed
---
