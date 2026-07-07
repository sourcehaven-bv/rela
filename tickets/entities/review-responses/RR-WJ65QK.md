---
id: RR-WJ65QK
type: review-response
title: 'Wire shape: return nil (not empty slice) for no-detail so omitempty drops it; render as escaped text only'
finding: 'MissingRequiredHeaders should return nil (not []string{}) when nothing is missing, and the content branch should set Detail only when len(missing)>0, so satisfied rules and non-content violations never emit detail:[] on the wire. Go omitempty drops both nil and empty slices -> TS detail?: string[] sees absence. On render, keep {{ }} interpolation / :title binding (auto-escaped); do NOT introduce v-html (frontend lints vue/no-v-html). Header names are metamodel-authored (lower risk) but still must not be raw-HTML-rendered.'
severity: nit
resolution: Plan steps 1/2 return nil (not []string{}) and set Detail only when len>0; omitempty drops it on the wire; step 6 renders via auto-escaped :title binding, no v-html.
status: addressed
---

See finding property.
