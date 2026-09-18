---
id: DEC-K3JYP0
type: decision
title: Remove `otherwise:`; exclusion is the only rule-3 behaviour
context: 'otherwise: default was designed against the pre-BUG-HC6I2T data model where a faced entity always had a bare row. Measured against ResolveWorldPrimes, the two otherwise: values are now indistinguishable for correctly faced data; they diverge only for stranded bare rows.'
consequences: 'otherwise: is rejected at load with a rela migrate step to remove it. stand_in: and on_absent.redirect survive. ResolutionAt''s terminal branch gains a new value, ResolutionOutsideWorld (wire: outside-world), because a face the world''s chain does not contain was selected by no resolution rule. Sequenced after BUG-UA3BK3 so operators can see stranded rows before removal hides them.'
date: "2026-09-18"
status: proposed
---

## Context

A world's `otherwise:` key declares what happens to an entity whose type
declares faces but which has none the world's chain names — "resolution rule 3".
It takes `exclude` (the entity is absent; absence IS the publication bit) or
`default` (fall back to the entity's default/zero-coordinate state). It is
currently **mandatory**: a world without it does not load.

`otherwise: default` was designed against a data model that no longer exists.
`FallbackDefaultState` landed 2026-08-20 (TKT-WAV8XP). BUG-HC6I2T removed the
headless-state invariant three weeks later (2026-09-10), so a type declaring
`faces:` no longer stores a row at the bare coordinate. The thing `default`
falls back **to** is normally not there.

## What the two values actually do today

Measured against `store.ResolveWorldPrimes`. For **correctly faced data**,
`exclude` and `default` are indistinguishable:

| case | exclude | default |
| --- | --- | --- |
| guide has `nl` (chain `[nl, en]`) | `face=nl via=chain` | same |
| guide has only `en` | `face=en via=chain` | same |
| guide has **neither** | ABSENT | ABSENT |
| policy unpublished (chain `[draft, published]`) | `face=draft via=chain` | same |

The third row matters: the worlds prototype's comment claims `site-nl`'s
`otherwise: default` is what makes "a guide with no Dutch face fall back to
English rather than disappearing", and calls that "the reason `otherwise:` is
mandatory rather than defaulted". That is a misreading of its own mechanism —
`select: [nl, en]` does the falling back. `otherwise:` contributes nothing
there.

Divergence needs **three** conditions at once: a faced type, a bare row, and no
chain face on that entity.

| case | exclude | default |
| --- | --- | --- |
| bare row only | ABSENT | `face="" via=fallback-default` |
| bare + `published`, chain wants `draft` | ABSENT | `face="" via=fallback-default` |
| bare + chain match | `face=draft via=chain` | same |

So `otherwise: default`'s only remaining function is **serving rows that should
not exist** — stranded data, which is BUG-UA3BK3's subject.

## Decision

Remove `otherwise:` as a metamodel key. Exclusion becomes the only rule-3
behaviour. Delete `metamodel.Otherwise`, `store.Fallback` /
`FallbackDefaultState`, `ResolutionFallbackDefault`, and the `via:
"fallback-default"` wire value.

## Consequences

**Kept.** `stand_in:` survives — it covers a within-chain fallback *and* an
`otherwise: default` substitution, and the first is still live (a positive
`chain_position`). `WorldBadge` loses one arm of a two-arm check.
`on_absent.redirect` is a separate SPA-level key, unaffected, and is arguably
the better answer to "no face here" than `otherwise: default` ever was.

**Lost.** An operator can no longer ask a world to serve a bare row as a
stand-in. That is intentional: after BUG-HC6I2T such a row is stranded, and the
right response is to migrate it (`migrate_face`) rather than to render it.

**Breaking, with a migration.** `otherwise:` is rejected at load with an error
naming the fix, and `rela migrate` (the schema-YAML migrator,
`internal/migration`) removes the key. Rejection is an ADDED check, not a
deletion: `WorldDef.UnmarshalYAML` does not set `KnownFields`, so without it the
key would be silently ignored and the world would flip from substitute to
exclude with no diagnostic. Silence was rejected for exactly that reason.

## `ResolutionAt`'s terminal branch is not a resolution rule

`store.ResolutionAt` is a total lookup, not a walk: it answers for a face
ALREADY chosen, where the family is not in hand. `internal/dataentry` calls it
for an entity the request **addressed by face** (`POL-1@draft` on
`?world=published` — what the face-switcher does), so resolution never ran.

Its terminal branch currently returns `ResolutionFallbackDefault`. Probed
against chain `[published]`:

| input | result |
| --- | --- |
| `published` (in chain) | `via=chain pos=0` |
| `draft` (off-chain, addressed directly) | `via=fallback-default` |
| bare face on a faced type | `via=fallback-default` |
| unscoped type | `via=unscoped` |

Under exclusion-only, resolution can never hand back an off-chain face — it
returns a chain coordinate or the entity is absent (verified). So the branch is
reachable **only when resolution did not run**. That is one condition, and it is
not one of the three rules: `ResolutionRule`'s own doc says it "names which of
the three world-resolution rules selected an entity's prime", and a
directly-addressed face was selected by none of them.

Neither existing value fits. `ResolutionExcluded` is false — the entity is on
screen, not absent. `ResolutionUnscoped` is false — the world does scope this
type. So the branch gets a **new** value, `ResolutionOutsideWorld`, spelled
`outside-world` on the wire: this face is not in the world's chain, so no rule
selected it.

The name describes the FACE'S RELATION TO THE WORLD, not how the request
arrived, and that is deliberate. The other three values are all facts about the
world's decision — `unscoped` made none, `chain` chose this one, `excluded`
rejected the entity — so `outside-world` ("not in its vocabulary at all") sits
in the same register.

`addressed` and `addressed-explicitly` were considered and rejected. Every face
in rela is addressed: `POL-1@draft` and `POL-1@published` are both addresses,
and a declared face name IS its storage coordinate. Worse, a directly-addressed
IN-CHAIN face (`POL-1@published` under `?world=published`) is equally explicit
and correctly reports `chain` — so "explicitly" is not the discriminator, and
naming it that way relocates the ambiguity rather than removing it. `off-chain`
is precise but leaks `select:`'s implementation vocabulary into an
operator-facing wire value and badge.

This is also **wrong today, independent of this decision**: a reader who opens
`POL-1@draft` under `?world=published` is told `via: "fallback-default"`, which
claims the `otherwise:` arm fired even in a deployment that declares `otherwise:
exclude`. Worth fixing as its own bug rather than folding in here.

The badge improves rather than degrades: `WorldBadge.isSubstitute` fires on the
new value, reading as "outside the published world" — more accurate for the
face-switcher than "a substitution happened".

The resulting vocabulary:

```text
via: "unscoped"       // world says nothing about this type
via: "chain"          // world chose this face (+ chain_position)
via: "excluded"       // world rejected this entity
via: "outside-world"  // face is not in this world's chain
```

## Alternatives rejected

- **Keep it, fix the client.** Teach the SPA the `otherwise:` dimension so
`worldForFace` can disambiguate. Rejected in TKT-Z4L0IU: it spreads resolution
semantics into the client and leaves the ambiguity real.
- **Keep `default` for mixed/migration states.** This is the one real use, but
it makes a transient state permanent and hides exactly the rows an operator
needs to see. Detection (BUG-UA3BK3) plus migration is the right pair.
- **Reuse `ResolutionExcluded` or `ResolutionUnscoped`** for the terminal
branch. Both describe a resolution outcome for something resolution never
touched. See above.

## Sequencing

1. **BUG-UA3BK3** — `rela analyze` reports stranded bare rows, so operators can
see them.
2. **BUG-TOX8U4** — `migrate_face` becomes transactional; it is the migration
path operators are being pointed at.
3. **This decision** — remove `otherwise:`, with the `rela migrate` step.

Removal must not precede detection: it hides rows that are currently visible,
and an operator with no way to see them has no way to act.

## One conformance test needs redesigning, not deleting

`storetest.RunWorldTests`' `FamiliesStayContiguousAcrossPageBoundaries` uses
`FallbackDefaultState` as a **discriminating verdict** — one where a partial
family view yields a WRONG prime rather than a missing one. Under exclusion the
duplicate never materialises, so the test keeps passing while guarding nothing
(its own comment says "with `exclude` both runs happen to agree, which is why
this case must use `default`").

Verified re-arm: give each entity both `draft` and `published` with `select:
[published, draft]` under exclude. A split family then emits the entity twice —
`draft` (position 1) on one run, `published` (position 0) on the other — so the
`yielded twice` fatal is live again, and the whole family still resolves to
`published`. The chain replaces the fallback as the discriminator.
