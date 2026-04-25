---
id: RR-J4GLF
type: review-response
title: 'Doc inaccuracy: missing-key fallback case omitted'
finding: 'GUIDE-metamodel.md says DisplayTitle falls back to ID ''only when the value is empty or nil.'' Implementation also falls back when the key is missing from the property map (val, ok := properties[primary]; !ok). Three fallback cases, two listed. Fix: ''falling back to the ID when the value is empty, missing, or nil.'''
severity: minor
resolution: 'GUIDE-metamodel.md ''Display name'' section was rewritten as part of the RR-IG4JJ / data-entry-doc-move work. Now states: ''The display falls back to the entity ID when the value is empty, missing, or nil.'' All three fallback cases listed.'
status: addressed
---
