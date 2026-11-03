---
id: RR-WHLM9D
type: review-response
title: Q5 refuse-vs-warn criterion is self-inconsistent and diverges from the choices precedent
finding: 'Q5 says refuse when a key can never have an effect and warn when a small edit would give it one. But confirm on a type with a required property can never have an effect either (the type button always opens the composer, channelView.ts:437-442) and the plan warns; and "no person-fired entry" is also fixed by a small edit (adding one entry) and the plan refuses. The precedent for a presentation key whose effect depends on the session graph is choices: it refuses only structural misuse (ephemeral, choices.go:28-31) and warns when the buttons can never show (choices.go:38-42). A refusal fails the whole file, so on reload deleting the last person-fired abandon entry would keep the old config because of a leftover cosmetic key. Change AC 3: refuse no-session and ephemeral (structural, as choices); warn for no person-fired entry, every entry buttoned, and confirm with a required property. Define person-fired with the same predicate as personClasses (buttons.go:118-128) so a type with no post role and no entry roles also counts as having no button.'
severity: significant
resolution: AC 4 now refuses only no session and an ephemeral type, and warns for no person-clickable entry (personClasses predicate) and for a confirm that reaches no one-click button. The every-entry-buttoned warning is dropped because inheritance gives the key an effect on presets.
status: addressed
---
