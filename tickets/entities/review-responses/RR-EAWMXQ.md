---
id: RR-EAWMXQ
type: review-response
title: Option text posted into a markdown `into` property renders differently from the button label
finding: 'AC 4 (plan line 142) allows `into` of type markdown. The button shows the option as plain text (step 9), but the posted answer is authored by the CLICKER and rendered as markdown: an option `[Yes](https://evil.example)` shows as a literal on the button and as a link labelled "Yes" in a message under the clicker''s name; `@alice` becomes a mention sent as the clicker (internal/realtime/mentions.go:91-104 scans markdown props). The question author chooses the text, the clicker gets attributed. The PoC answer type uses `body: string` (rela-agent-runner-poc realtime.yaml:54), so nothing needs markdown now. Fix: allow `into` of type `string` only in this slice (refuse markdown at `.into`); if markdown is wanted later, escape the option as a markdown literal before posting and pin it with a test.'
severity: significant
resolution: into must be a string property; markdown is refused at .into (AC 4), and the Security section explains the attribution reason.
status: addressed
---
