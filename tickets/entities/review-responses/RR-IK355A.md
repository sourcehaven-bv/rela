---
id: RR-IK355A
type: review-response
title: Whitespace-padded asserted_role_assignments key loads clean but is permanently unmatchable
finding: 'Validate rejected a claim key only when isBlank (all-blank), but computeGlobals does strings.TrimSpace on the incoming claim and then an EXACT map lookup against the untrimmed stored key. So `asserted_role_assignments: {" admin": editor}` loaded clean and was permanently inert — no claim value could ever match it. Reproduced independently: Validate ACCEPTED '' admin'', and claims "admin", " admin", "admin " all yielded 0 attributions. It fails safe (a grant silently does not happen) but it is exactly the looks-configured-but-does-nothing trap Validate''s own blank-key check exists to prevent, and EffectiveMembershipRelation (policy.go:158-163) already trims its value for the identical reason. Second-order: isBlank only treats ASCII space/tab/CR/LF as blank while TrimSpace also strips Unicode spaces, so an NBSP-only key passed Validate and was likewise inert.'
severity: significant
resolution: 'Fixed by normalizing at load rather than patching the comparison. Policy.normalizeAssertedRoles trims every key and is called from Policy.Validate — deliberately in Validate, not LoadPolicyBytes, so a Policy reaching the resolver through any other construction path is normalized too. An all-blank key trims to "" and is then caught by the existing blank-key check, so it still fails loudly instead of normalizing into something matchable; the Unicode-space case now collapses the same way. Keys differing only by padding merge, and their role lists are unioned so no grant is silently dropped. Two regression tests (TestAssertedRoles_PaddedClaimKeyIsNormalized, _PaddedKeysMergeRatherThanClobber) were fault-injected: with normalizeAssertedRoles removed they fail with ''the key was stored untrimmed and is unmatchable'' and ''role "auditor" lost when padded keys merged''.'
status: addressed
---

## Finding

`Validate` (`policy.go:466-472`) rejected a claim key only when `isBlank` — all
characters blank. But `computeGlobals` (`resolver.go:57-59`) trims the
**incoming claim** and then does an **exact** lookup against the **untrimmed
stored key**.

Reproduced independently before fixing:

```text
Validate ACCEPTED key ' admin'
claim "admin"  -> 0 attributions
claim " admin" -> 0 attributions
claim "admin " -> 0 attributions
```

A padded key is therefore **permanently inert** — no claim value can ever match
it.

This fails *safe* (a grant silently doesn't happen), which is why it is
significant rather than critical. But it is precisely the
looks-configured-but-does-nothing trap `Validate`'s blank-key check exists to
prevent, and the surrounding code already knows the answer:
`EffectiveMembershipRelation` (`policy.go:158-163`) trims its value for the
identical reason.

Second-order: `isBlank` (`policy.go:593-599`) treats only ASCII space, tab, CR
and LF as blank, while `TrimSpace` also strips Unicode spaces. An NBSP-only key
passed `Validate` and was likewise inert.

## Resolution

Normalized at load rather than patching the comparison:
`Policy.normalizeAssertedRoles` trims every key, called from `Policy.Validate` —
deliberately there and not in `LoadPolicyBytes`, so a `Policy` reaching the
resolver through any other construction path is normalized too.

- An all-blank key trims to `""` and is caught by the existing blank-key
check, so it still fails loudly rather than normalizing into something
matchable.
- The Unicode-space case collapses the same way.
- Keys differing only by padding merge; their role lists are **unioned**, so
no grant is silently dropped.

Both regression tests were fault-injected. With `normalizeAssertedRoles` removed
they fail with *"the key was stored untrimmed and is unmatchable"* and *"role
\"auditor\" lost when padded keys merged"*.
