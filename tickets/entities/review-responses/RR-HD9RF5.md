---
id: RR-HD9RF5
type: review-response
title: Button label is not NFC-normalized but is compared NFC-folded
finding: 'internal/realtimecfg/buttons.go:84 stores TrimSpace(DropStyleSelectors(label)) and counts runes before NFC. states() compares foldDisplay(label), which applies NFC. A decomposed label can fail the 100-rune limit while its NFC form fits, and the served label differs in form from what was compared. Fix: use foldDisplay(raw.Label) at :84, as normalizeListItem does.'
severity: nit
resolution: The label is now foldDisplay(raw.Label), so it is NFC before the length check and is served in NFC. Added a test with 100 decomposed characters that loads and is stored in NFC.
status: addressed
---
