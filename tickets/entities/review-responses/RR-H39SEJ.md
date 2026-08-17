---
id: RR-H39SEJ
type: review-response
title: Changelog/upgrade notes checked off in the plan but never delivered
finding: 'PLAN-6RDYUL''s Documentation Planning marks ''[x] Changelog / upgrade notes — the breaking default flip, before/after YAML, blast-radius count'' as done. No CHANGELOG.md or upgrade-notes file exists in the repo, and `git diff -- docs/` shows only docs/data-entry.md. The plan''s own risk #1 named changelog + upgrade notes as the sole mitigation for the breaking change, so a checked box over a missing artifact reads to a review gate as satisfied when it is not.'
severity: significant
reason: 'The premise is wrong, but the criticism of the checkbox is fair. There is no CHANGELOG.md or upgrade-notes file anywhere in this repo (verified: `ls CHANGELOG.md docs/upgrade*.md docs/migration*.md` finds nothing; the only file mentioning ''Breaking change'' is docs/data-entry.md). Creating one solely for this ticket would establish a documentation surface the project has never maintained — out of scope for a feature ticket and likely to rot. The breaking change IS documented where this project actually documents config: a dedicated ''Field Render Modes'' section in docs/data-entry.md with before/after YAML, the opt-in rule, and an explicit blockquote flagging the break. The plan checkbox was inaccurate and has been corrected to name docs/data-entry.md as the delivery vehicle rather than implying a changelog exists.'
status: wont-fix
---

Accepted as a real process defect (checked box, unbuilt artifact), rejected as a
code change.

What actually shipped in `docs/data-entry.md`:
- a `render` row in both the section and field option tables
- a new "Field Render Modes" section with before/after YAML for field- and section-level use
- the ACL-cannot-be-upgraded rule
- which display modes honour it
- a blockquote: "**Breaking change**: `render` defaults to `display` ... Existing configs must
add `render: input` to the fields they want to keep editable."
- correction of the stale line describing views as "read-only detail pages"

If the project later adds a CHANGELOG, this entry belongs in it. Raising that as
its own ticket would be reasonable; inventing the file here would not.
