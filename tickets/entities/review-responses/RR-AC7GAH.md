---
id: RR-AC7GAH
type: review-response
title: World-name charset is a security control, not hygiene — and the loader comment claiming it is handled is wrong
finding: |-
    VERIFIED. dn37j2-plan.md §6 frames the tightening as 'hygiene, correctly sequenced'. Understated.

    ValidateSchemaName (metamodel/validation.go:43-56) blocks ONLY: empty, single-quote, backslash, \x00-\x1f, \x7f, and leading/trailing whitespace. It PERMITS `/`, `..`, `%`, `?`, `#`, `&`, `=`, `:`, double-quote, internal spaces, and all non-ASCII (incl. U+2028/U+2029 and RTL overrides).

    So `../admin`, `a b`, `pub%2f`, `world?x=1` all load clean TODAY. If world names ever become URL PATH segments rather than query values, `..` is a traversal primitive.

    The loader comment at metamodel/loader.go:716-719 already asserts this is handled — 'A world name reaches URLs, acl.yaml and CLI flags in later steps. The empty name is the dangerous one.' That comment is WRONG ON ITS OWN TERMS: empty is not the only dangerous name, it is merely the only one currently blocked. Fix the comment along with the check.

    The plan's proposed grammar [a-z][a-z0-9]*(-[a-z0-9]+)* is correct and matches entity.facePattern (entity/face.go:58) exactly — good.

    TWO REQUIREMENTS THE PLAN OMITS:
    1. Do NOT write a third copy of the regex. Reuse/export entity's pattern; confirm arch-lint allows metamodel -> entity (it currently does NOT: metamodel mayDependOn is {migration, storage}). If the boundary blocks it, validate in internal/worlds like Q6 of Step 2 did for face grammar — there is precedent.
    2. §6 asserts 'no shipped or fixture metamodel declares a world outside the grammar'. VERIFY before landing, including prototypes/ and docs-project/.
severity: significant
status: open
---
