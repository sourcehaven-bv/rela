---
id: RR-Y0UN58
type: review-response
title: 'Justification rests on a falsified claim: the fallback arm IS reached in-tree'
finding: 'The commit, its doc comment, the test comment and TKT-Z4L0IU all justified dropping `otherwise` from `primacyKey` by asserting that `ResolveWorldPrimes`''s `FallbackDefaultState` arm is unreachable for a faced type (guarded on `f.haveDefault`; BUG-HC6I2T removed the requirement that a named face carry a zero-coordinate row). That generalises from a probe run against a constructed faced-only entity. It is false: a type may still hold bare rows, and `prototypes/perf/project` is an in-tree fixture where it does. `perfseed` writes every policy at the bare coordinate plus an optional `published` face and never a `draft` row, so the `editorial` world (chain [draft, published], otherwise: default) takes the fallback arm. Both the premise (no bare rows for faced types) and the conclusion (the two `otherwise:` values coincide) are contradicted by a fixture in the same repository.'
severity: significant
resolution: 'Verified the reviewer''s claim directly against `store.ResolveWorldPrimes` using the perf fixture''s shape: one bare policy row with chain [draft, published] gives `map[]` under `exclude` and `{POL-1: {Face:"", Via:2}}` under `default` — they differ observably. Rewrote the justification in `loader.go`, `faceprimacy_test.go` and the ticket to rest on the per-type/per-face argument alone: the primacy loop reaches the rule only for a face the type declares and both worlds lead, and a reader switching to a face they HAVE is served by the chain head, never by `otherwise:`. BUG-HC6I2T is demoted to a corroborating aside with its precondition (all rows faced) stated. The code change itself is unaffected.'
status: addressed
---

The change stands; only its rationale was wrong. The reviewer's independent
reading of the loop reached the same conclusion the fix now rests on, so no code
moved — but the false claim would have misled the next person to touch this
rule, and would have been cited as precedent in the follow-up that removes
`otherwise:` outright (where it is load-bearing and where bare-row types
genuinely change behaviour).

Worth recording as a process point: the probe was run against a case I
constructed to match the theory, rather than against the fixtures in the repo
that could have falsified it. The perf fixture was one `grep` away.
