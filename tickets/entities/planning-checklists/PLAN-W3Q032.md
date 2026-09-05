---
id: PLAN-W3Q032
type: planning-checklist
title: 'Planning: Skip scheduled mail when no section has content visible to the recipient'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Declarative scheduled mail renders per recipient through the recipient-scoped
visible reader, but sends unconditionally. When ACL row-gating removes every
matched entity, the recipient gets a message whose sections all render "Nothing
to show." (`internal/mailrender/template.go:225` for HTML,
`internal/mailrender/text.go:82` for the plain-text alternative).

**Scope:**

IN:

- `require_visible_content bool` on `mailtemplate.Template`, parsed with the
existing `KnownFields(true)` decoder.
- A way for `RunScheduledTemplate` to learn the matched-entity count, which
`mailtemplate.Build` computes today but discards.
- Suppression in `Services.RunScheduledTemplate` before `sender.Send`.
- An operator-facing log line on suppression.
- Tests per the criteria below.

OUT:

- Relational filters in `for_each.where` — the originating request, rejected
in the ticket body with reasoning and file:line citations.
- Any change to `internal/predicate`, `internal/acl`, or
`internal/visibility`.
- Automation-triggered mail (TKT-LU4AAY).
- Making the `for_each` selection cheaper. Fan-out still visits every
candidate; this ticket changes what is *sent*, not what is *rendered*.

**Acceptance Criteria:**

1. Recipient-scoped ACL filtering unchanged — same visible reader renders
content; the raw recipient record still only addresses the envelope. *Test:*
existing `TestRunScheduledTemplate_RedactsPerRecipientACL` and
`_RedactsFieldOnAVisibleRow` continue to pass untouched.
2. No mail sent when every section is empty after ACL filtering and
`require_visible_content: true`. *Test:* `alice` (denied `task` rows) + opted-in
template → `sender.Messages()` stays empty.
3. Existing configs unchanged by default.
*Test:* same fixture with the key absent → message IS sent and still contains
"Nothing to show.".
4. A suppressed send emits a diagnostic log line (Info) naming template and
recipient and NOTHING else. *Test:* capture `slog` output; assert it names
template + recipient and does NOT contain the hidden entity's title or property
values. Info is correct because suppression is configured, non-actionable
behaviour — see RR-0W5FHK; this is a diagnostic for an operator who raises the
level, not a default-visible mitigation.
5. Tests cover fully hidden, partially visible, fully visible, and opted-out.

## Research

- [x] For larger features: run `/research` — N/A, effort `s`, single call site
- [x] Searched for existing libraries — N/A, no external dependency
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects — N/A
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, self-contained change with one production call
site.

**Existing Solutions:**

- `mailtemplate.Build` (`internal/mailtemplate/mailtemplate.go:80`) already
computes `count` — matched entities across all sections — as a local, used only
to expand `{{count}}` in subject and intro. It is computed on the ACL-filtered
side of the reader, which is the right place, but it counts MATCHES rather than
CONTENT — see RR-K7RMIC. A second counter is added alongside it.
- `Build` has exactly ONE production caller,
`internal/appbuild/scheduled_mail.go:60`. A signature change is cheap and fully
surveyed (`projectsetup/validate.go` and `scheduled_mail_validate.go` call
`Parse`, not `Build`).
- `internal/appbuild/scheduled_mail_acl_test.go` already builds the exact
fixture needed: `alice` holds `viewer` (no read on `task` → zero visible rows),
`dana` holds `lead` (all rows + the `secret` field). Fully-hidden and
fully-visible cases exist; only partially-visible needs adding.
- Precedent for opt-in template keys: `Section.Style` / `Section.Link` are
parsed and validated the same way in `mailtemplate.Parse`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

The one real design decision is how `RunScheduledTemplate` learns the count.
`Build` returns `*mailrender.Message`, and `Message` (`mailrender.go:97`) has no
count field — nor should it, since `mailrender` is a pure formatting leaf that
must not learn about ACL or matching.

Chosen: **change `Build` to return `(*mailrender.Message, int, error)`**, the
int being the number of entities that CONTRIBUTED CONTENT to a section — not the
number that matched (RR-K7RMIC).

Contribution is decided in the same loop, per style:

- `table` (and default, i.e. empty style): always contributes — a row is appended.
- `list`: always contributes — a link line is written.
- `detail`: contributes only when `strings.TrimSpace(ent.Content) != ""`.

This is a SECOND counter. The existing `count` keeps its current meaning
("entities matched") because `{{count}}` legitimately interpolates that, and
changing it would silently alter every template that uses it.

- Honest and local: the caller that owns the send decision gets the fact, and
the renderer stays a formatter.
- One call site to update.
- Keeps `mailrender.Message` free of a field only the scheduler would read.

Then in `RunScheduledTemplate`, after `Build` and before rendering:

```go
model, contributed, err := mailtemplate.Build(ctx, s.meta, deps.VisibleReader, tmpl, time.Now())
if err != nil {
    return err
}
if tmpl.RequireVisibleContent && contributed == 0 {
    slog.InfoContext(ctx, "scheduled mail has no visible content; skipping",
        "template", name, "recipient", recipientID)
    return nil
}
```

Placed before `mailrender.New`/`Render` so a suppressed message is never
rendered — cheaper, and it keeps untrusted content out of the sanitizer on a
path whose output is discarded.

Returning `nil` (not an error) matches the established convention on this path:
`skipBadAddress` (`scheduled_mail.go:77`) and the `ErrNotFound` recipient branch
both return `nil` for "nothing to do". An error would mark the child job failed;
template child jobs are enqueued with `jobs.RetryBounded`
(`internal/scheduler/foreach.go:87`), so it would re-render on every attempt and
never succeed. Note the policy is not uniform — the scheduler's own task
submissions use `jobs.RetryNever` because "the scheduler owns retrying"
(`internal/scheduler/jobs.go:203`) — so cite foreach.go:87, not a blanket rule
(RR-3NKKAP).

**Alternatives considered:**

- *Inspect the rendered `mailrender.Message`* — reject. Emptiness is spread
across two fields depending on style: `detail`/`list` accumulate `Section.Body`,
`table` accumulates `Section.Rows`. A predicate over those would silently
misjudge a `detail` section whose entity has empty `Content`, and would have to
be kept in sync with every future style.
- *Add a `Count` field to `mailrender.Message`* — reject. Pushes a
matching/ACL concern into a pure formatting leaf that has no other reason to
know about entities.
- *Put the key on `for_each` (as originally specified)* — reject. `for_each`
is generic fan-out that also serves `script:` tasks with no template, where the
key would be silently inert. On `Template` it is inert by construction.
- *Recompute the count in `RunScheduledTemplate`* — reject. Duplicates the
filter/ACL loop; the two copies would drift.

**Files to modify:**

- `internal/mailtemplate/mailtemplate.go` — add
`RequireVisibleContent bool
\`yaml:"require_visible_content,omitempty"\``to`Template`; change `Build` to
return the count.
- `internal/appbuild/scheduled_mail.go` — consume the count, suppress, log.
- `internal/mailtemplate/mailtemplate_test.go` — parse + Build count tests.
- `internal/appbuild/scheduled_mail_acl_test.go` — suppression tests reusing
the existing ACL fixture.
- `docs/` — the scheduled-mail page (identify exact file during implementation).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `require_visible_content` — operator-authored `mail-templates.yaml`. A bool;
YAML type mismatch is rejected by the existing `dec.KnownFields(true)` decoder
in `Parse`, which already fails closed on unknown keys. Per CLAUDE.md ("The
configuration is not a secret"), this is operator config in the repo — no
per-principal concealment applies to the key itself.
- No new untrusted input. Entity content still flows through the unchanged
`mailrender` pipeline (markdown → goldmark → bluemonday on content only →
trusted template → douceur inline last).

**Security-Sensitive Operations:**

- **The ACL read path is untouched.** `Build` still receives
`deps.VisibleReader`; the count is derived from what that reader already
yielded. Suppression reads no additional data and widens nothing.
- **The suppression log must not leak.** AC 4 requires the operator to see
*that* a send was skipped; it must NOT name the entities that were filtered.
Logging template name + recipient ID only. Both are non-secret: the template
name is operator config, and the recipient ID is already logged by
`skipBadAddress` on the adjacent path.
- **One-bit inference is accepted and pre-existing.** A recipient can infer
"I have no visible matching entities" from the mail's absence — the same bit
they already get from a "Nothing to show." message. No *values* are disclosed.
This matches the documented row-level rule: hidden entities are
indistinguishable from nonexistent ones, and membership is an accepted channel.
- Suppression happens before rendering, so a discarded message's untrusted
content is never sanitized or inlined.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Level |
| --- | --- | --- |
| 1 | Existing `_RedactsPerRecipientACL` / `_RedactsFieldOnAVisibleRow` pass unmodified | integration |
| 2 | `alice` + `require_visible_content: true` → `sender.Messages()` empty | integration |
| 3 | `alice` + key absent → message sent, contains "Nothing to show." | integration |
| 4 | `slog` capture on suppression names template+recipient, excludes hidden title/values | integration |
| 5 | fully hidden / partial / fully visible / opted-out matrix | integration |

Integration tests go in `internal/appbuild/scheduled_mail_acl_test.go` and drive
the real `RunScheduledTemplate` against `memstore` + a real `acl.Declarative` +
`mail.NewMemorySender` — the full path, not a mock.

Unit tests in `internal/mailtemplate/mailtemplate_test.go` cover `Parse`
round-tripping the new key and `Build` returning an accurate count.

**Edge Cases:**

- Template with zero sections → count 0. Opted in: suppressed. Opted out:
sent (may carry a meaningful `intro`). Documented as deliberate.
- Multi-section, one non-empty → count > 0 → sent. Guards against an
any-section-empty misreading of the criterion.
- `detail` section whose matched entity has empty `Content` → contributes 0, so
SUPPRESSED when opted in (RR-K7RMIC). This is the case the original plan got
wrong: it would have sent a message rendering as "Nothing to show.", exactly
what the ticket exists to prevent. Also assert `{{count}}` still expands to `1`
in the opted-out case, pinning that the two counters are genuinely distinct.
- Section matching entities where all are ACL-hidden, alongside a section with
visible rows → sent.
- Opted in + recipient with no address → existing `skipBadAddress` wins first
(it precedes `Build`); no interaction.

**Negative Tests:**

Verified empirically against `gopkg.in/yaml.v3` before writing these — the
intuitive expectation is wrong (RR-NV7O2V):

- `require_visible_content: "true"` (QUOTED) → `Parse` returns an error
(`cannot unmarshal !!str into bool`). Pin this: quoting is the spelling an
operator most likely reaches for, and the failure is loud.
- `require_visible_content: yes` and `"yes"` → both accepted as `true`
(YAML 1.1 booleans). Pin as documented behaviour so a yaml library bump that
changes it is caught.
- Unknown neighbouring key stays rejected by `KnownFields(true)` (regression
guard that the struct change didn't loosen the decoder).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Silent non-delivery is harder to diagnose.* ACCEPTED, not mitigated
(RR-0W5FHK). "Why didn't I get the mail?" now has two indistinguishable causes:
no matching data, or no *visible* matching data. The AC 4 log line helps an
operator who raises the log level, but it is Info and therefore not visible by
default — correctly so, since suppression is configured, non-actionable
behaviour.
- *`Build` signature change.* Mitigated by survey: one production caller,
compiler-enforced. Test callers update mechanically.
- *Does not itself restrict anything.* If `mt_overleg` is readable org-wide,
every recipient sees content and nobody is suppressed. This is a routing
mechanism only where visibility is ALREADY scoped. Carried from the ticket
because it decides whether this is sufficient or merely necessary for the
originating Atlas use case — **open question for the user, does not block
implementation.**
- *Cost unchanged.* Still renders per recipient to discover most are empty.
`ForEachConfig.EffectiveLimit()` bounds children; unchanged here.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] Scheduled-mail docs — new `require_visible_content` key: semantics
(matched entities, not rendered bytes), default-off, and the diagnostic note
that a suppressed send is logged. Exact file identified during implementation
(DOCS-MAILD1 from TKT-U2R7GU indicates a declarative-scheduled-mail page).
- [x] ~~docs/metamodel.md~~ (N/A: not a metamodel feature)
- [x] ~~docs/cli-reference.md~~ (N/A: no command added or changed)
- [x] ~~docs/data-entry.md~~ (N/A: no UI surface)
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting pattern)
- [x] ~~README.md~~ (N/A: not a project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Summary | State |
| --- | --- | --- | --- |
| RR-K7RMIC | significant | Counting MATCHES sends an empty `detail` mail — the exact bug the ticket fixes. Count CONTRIBUTIONS instead, as a second counter so `{{count}}` is unchanged. | plan updated |
| RR-NV7O2V | significant | Negative test asserted YAML behaviour yaml.v3 does not have; verified empirically, prediction was inverted (`"true"` errors, `"yes"` succeeds). | plan updated |
| RR-3NKKAP | minor | Retry rationale implied a uniform policy; cite `foreach.go:87` (`RetryBounded` for child jobs) vs `jobs.go:203` (`RetryNever` for submissions). | plan updated |
| RR-0W5FHK | minor | Original finding was itself wrong — argued for `Warn` to satisfy AC 4. Level tracks actionability; suppression is configured and non-actionable. Kept Info, fixed AC 4's over-claim instead. | addressed |

No critical findings. Two significant findings were defects in the plan's own
reasoning, both corrected above before any code was written.
