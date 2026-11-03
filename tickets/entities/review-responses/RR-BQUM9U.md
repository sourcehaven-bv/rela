---
id: RR-BQUM9U
type: review-response
title: Invisible and bidi characters written as raw literals in Go source
finding: 'internal/texthazard/texthazard.go:26 and :30 compare against raw U+200D, U+2028, U+2029, U+034F and the Hangul fillers. texthazard_test.go:17-31 holds raw U+200D, U+200B, U+202E (RLO), U+061C, U+00AD, U+2028 and U+2029. listprop_test.go:65 and :91 build the NFC cases from a raw decomposed e+U+0301 next to a raw U+00E9. Nobody can review a set they cannot see, GitHub flags the files as hidden or bidirectional Unicode (a Trojan Source pattern), and an editor that normalizes to NFC silently turns the NFC tests into trivial ones. Fix: write every such rune as an escape (''‍'', ''     '', ''‮'', "é" against "é") and add a bidichk-style check, or a test that greps these packages for raw Cf, Zl and Zp runes.'
severity: significant
resolution: Every invisible, bidi, variation-selector, Hangul-filler and combining rune in texthazard.go, texthazard_test.go and listprop_test.go is now a \u escape (also the pre-existing raw U+0301 in realtime/mentions_test.go). New TestSourceHasNoRawHazards in internal/texthazard fails on any raw display hazard or combining mark (Mn) in the Go source of texthazard, realtime and realtimecfg.
status: addressed
---
