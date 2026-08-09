---
id: RR-TD74AU
type: review-response
title: Redacted reaches command-script stdin — an undocumented addition to the command input contract
finding: 'commandInput (dataentry/commands.go:223-227) embeds the DOMAIN entity.Entity directly and marshals it whole at line 454, so the new Redacted field ships on command-script stdin. buildViewInput receives the viewResult whose Entry was redacted at views.go:65. Reviewer confirmed empirically: {"context":"view","entity":{...,"redacted":["salary"]}}. This is the only production surface where the field escapes the process, and it was an unversioned change to a documented input contract that nobody had reviewed as such.'
severity: significant
resolution: 'Kept deliberately and documented, rather than suppressed with json:"-". Rationale: (1) the field is genuinely useful there — a command hitting a stripped property otherwise cannot tell ''withheld'' from ''never set'', the same problem this ticket exists to solve for Lua; (2) it is names-only, matching the settled disclosure boundary; (3) it follows an EXISTING precedent on the identical surface — entity.Inaccessible already ships on command stdin by the same embedding, verified by marshalling a domain entity carrying both. Documented in docs-project GUIDE-data-entry.md (regenerated to docs/data-entry.md) with a worked JSON example, the redacted-vs-inaccessible distinction, and a warning not to echo either into a write. Pinned by TestBuildEntityInput_CarriesRedactedNames, which asserts the NAMES ship and the VALUE does not.'
status: addressed
---

## Finding (from cranky-code-reviewer)

`internal/dataentry/commands.go:223-227`:

```go
type commandInput struct {
    Context     string                      `json:"context"`
    Entity      *entity.Entity              `json:"entity,omitempty"`
    Entities    []*entity.Entity            `json:"entities,omitempty"`
    Collections map[string][]*entity.Entity `json:"collections,omitempty"`
    ...
}
```

The domain type is embedded and marshalled whole (line 454), so `redacted`
reaches command scripts. I had reasoned about the v1 wire serializers and the
MCP DTO and concluded "explicit DTOs everywhere" — missing that this surface has
no DTO at all.

## Decision: keep it, document it

Three reasons for keeping rather than adding `json:"-"`:

1. **It is useful exactly here.** A command that renders or exports an
entity faces the same ambiguity the ticket exists to fix. Suppressing it would
leave command scripts as the one consumer that still cannot tell withheld from
unset.
2. **Names-only**, within the settled disclosure boundary. The values
are stripped from `properties` before the payload is built.
3. **Precedent already exists on this exact surface.** `Inaccessible`
ships there today by the same embedding. Verified:

   ```json
   {"id":"P-1","type":"person","properties":{"name":"Ann"},
    "inaccessible":[{"name":"content","reason":"git-crypt"}],
    "redacted":["salary"]}
   ```

So this is consistent with the established shape, not a new channel.

## What was added

- Docs: a "Redacted and inaccessible properties" subsection under the
command scopes in `docs-project/entities/guides/GUIDE-data-entry.md` (the
generated-docs SOURCE), with a worked example, the redacted-vs-inaccessible
distinction, and a warning that a command writing entities back must not echo
either field into the write.
- Test: `TestBuildEntityInput_CarriesRedactedNames` pins that the names
ship and the value does not — making this a deliberate contract rather than an
accident of struct embedding.

## Note for future work

`commandInput` embedding the domain type means *any* future field on
`entity.Entity` silently joins this contract. That is a latent hazard beyond
this ticket; worth a DTO if the struct grows further.
