---
id: RR-1QWV9S
type: review-response
title: select() performs no type coercion — the Side numeric invariant is enforced only by a template binding
finding: 'parseSide() goes to deliberate trouble to return a number rather than a numeric string (useVersionSelectionSync.ts:76-87), because the <option value="current"> is a string while version options bind :value="m.version" (a number), so a string ''3'' matches no option and v-model blanks the dropdown. But select() (:140-145) writes next.base straight into the ref with no validation: select({base: ''2''}) leaves base.value === ''2'', serializes to a plausible-looking URL, and blanks the control. Today this is safe only because onBaseChange passes baseSel.value, which v-model already populated as a number — i.e. the type discipline lives in a Vue template rather than in the composable that owns the type. A leaked string IS repaired by the next seedFromUrl(), so the damage is a blanked dropdown until reload, not permanent.'
severity: significant
resolution: 'Fixed. Added an exported coerceSide(s) helper and applied it on every write into the refs: select(), the new resetToDefaults(), and the defaults fallback inside seedFromUrl(). The numeric invariant now lives with the type in the composable rather than depending on each caller (previously it held only because v-model happened to supply numbers from :value="m.version"). Three unit tests pin the contract: a numeric string passed to select() coerces to a number, a numeric string coming from a view''s defaults() coerces, and the ''current'' sentinel is left untouched. parseSide is unchanged — it already guaranteed this for URL-sourced values.'
status: addressed
---

## Finding

The composable owns the `Side` type and its numeric invariant, but only
`parseSide` enforces it. `select()` is an unguarded write.

```ts
select({ base: '2' as unknown as Side })  // base.value === '2', dropdown blanks
```

## Why it matters

The mutation test I ran proves the *current* wiring is correct. It does not
prove the *next* caller will be. The invariant should live with the type, not in
a template binding two files away.

## Fix

Coerce in `select()`, three lines:

```ts
function coerce(s: Side): Side {
  return s === CURRENT ? CURRENT : Number(s)
}
```

Plus a unit test pinning the contract (the existing e2e assertion only catches
the current wiring).
